package onedep

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
)

// extracts the id of the deposition from the response
func decodeDid(resp *http.Response) (string, error) {
	type DidType struct {
		Did string `json:"id"`
	}
	var d DidType
	decoder := json.NewDecoder(resp.Body)
	err := decoder.Decode(&d)

	if err != nil {
		return "", fmt.Errorf("could not decode id from deposition entry: %v", err)
	}
	return d.Did, nil
}

// extracts the id of the file from the response
func decodeFid(resp *http.Response) (string, error) {
	type FidType struct {
		Fid int32 `json:"id"`
	}
	var f FidType
	decoder := json.NewDecoder(resp.Body)
	_ = decoder.Decode(&f)
	var result map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	// errors point to the entries still missing for the whole deposition and do not represent the error for this exact request
	// if err != nil {
	// 	 return "", fmt.Errorf("could not decode id from deposition entry: %v", err)
	// }

	return fmt.Sprintf("%d", f.Fid), nil
}

func decodeResponse(resp *http.Response) ResponseType {

	type ResponseOneDep struct {
		Code    string           `json:"code"`
		Message string           `json:"message"`
		Extras  map[string][]any `json:"extras,omitempty"` // should add this?
	}
	var rOneDep ResponseOneDep
	decoder := json.NewDecoder(resp.Body)
	err := decoder.Decode(&rOneDep)
	if err != nil {
		return ResponseType{
			Status:  "cannot_decode",
			Message: err.Error(),
		}
	}
	fmt.Println(rOneDep.Extras)
	return ResponseType{
		Status:  rOneDep.Code,
		Message: fmt.Sprintf(rOneDep.Message),
	}
}

// reads the header of mrc files and extracts the pixel spacing
func getMeta(file multipart.File) ([3]float32, error) {
	var pixelSpacing [3]float32
	// https://bio3d.colorado.edu/imod/betaDoc/mrc_format.txt
	// words I care about: Mode(4),	sampling along axes of unit cell (8-10), cell dimensions in angstroms(11-13) --> pixel spacing = cell dim/sampling
	header := make([]byte, headerSize)
	_, err := file.Read(header)
	if err != nil {
		return pixelSpacing, fmt.Errorf("failed to read header: %v", err)
	}
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return pixelSpacing, err
	}
	var mode uint32 = binary.LittleEndian.Uint32(header[modeWord*wordSize : modeWord*wordSize+wordSize])
	var cellDim [3]float32
	if castFunc, ok := typeMap[mode]; ok {
		for i := 0; i < 3; i++ {
			cellDim[i] = castFunc(header[(cellDimWord+i)*wordSize : (cellDimWord+i)*wordSize+wordSize]).(float32)
			sampling := binary.LittleEndian.Uint32(header[(samplingWord+i)*wordSize : (samplingWord+i)*wordSize+wordSize])
			// Calculate pixel spacing
			pixelSpacing[i] = cellDim[i] / float32(sampling)
		}
	} else {
		return pixelSpacing, fmt.Errorf("mode in the header is not described in EM community: %v", err)
	}
	return pixelSpacing, nil
}

// sends a request to OneDep to create a new deposition
func CreateDeposition(client *http.Client, userInput UserInfo, token string) (Deposition, *ResponseType) {
	var deposition Deposition
	// Convert the user input to JSON
	jsonInput, err := json.Marshal(userInput)
	if err != nil {
		return deposition, &ResponseType{
			Status:  "error parsing request body",
			Message: err.Error(),
		}
	}
	url := baseURL + "new"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonInput))
	if err != nil {
		return deposition, &ResponseType{
			Status:  fmt.Sprintf("error sending request to url %v", url),
			Message: err.Error(),
		}
	}

	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return deposition, &ResponseType{
			Status:  "error sending request to the server",
			Message: err.Error(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 201 { // even not created instance, when e.g country is set wrong this seems to return a 200 with an error.
		// if responseDecoded.Message == "" { // now it would return  200 status code for hen there is an error when country is specified wrong
		deposition.Id, err = decodeDid(resp)
		if err != nil {
			return deposition, &ResponseType{
				Status:  "couldn't decode deposition id",
				Message: err.Error(),
			}
		}

		return deposition, nil
	}
	responseDecoded := decodeResponse(resp)
	return deposition, &responseDecoded
}

// prepare deposition instance, body request and multipart writer
func prepareFileDeposition(deposition string, fileUpload FileUpload) (DepositionFile, *bytes.Buffer, *multipart.Writer, error) {
	var fD DepositionFile
	fD.DId = deposition
	fD.Name = fileUpload.Name
	fD.Type = fileUpload.Type
	fD.ContourLevel = fileUpload.Contour
	fD.Details = fileUpload.Details

	// create body
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	err := writer.WriteField("name", fD.Name)
	if err != nil {
		return fD, body, writer, err
	}
	err = writer.WriteField("type", fD.Type)
	if err != nil {
		return fD, body, writer, err
	}
	return fD, body, writer, err
}

// sends a request to OneDep to add files to an existing deposition with id
func AddCIFtoDeposition(client *http.Client, deposition string, fileUpload FileUpload, file string, token string) (DepositionFile, *ResponseType) {
	fD, body, writer, err := prepareFileDeposition(deposition, fileUpload)
	if err != nil {
		return fD, &ResponseType{
			Status:  "request_body_issue",
			Message: err.Error(),
		}
	}
	// open file
	cifFile, err := os.Open(file)
	if err != nil {
		return fD, &ResponseType{
			Status:  "cif_file_issue",
			Message: err.Error(),
		}
	}
	defer cifFile.Close()

	//upload files
	part, err := writer.CreateFormFile("file", fD.Name)
	if err != nil {
		return fD, &ResponseType{
			Status:  "request_form_issue",
			Message: err.Error(),
		}
	}

	_, err = io.Copy(part, cifFile)
	if err != nil {
		return fD, &ResponseType{
			Status:  "request_form_issue",
			Message: err.Error(),
		}
	}

	err = writer.Close()
	if err != nil {
		return fD, &ResponseType{
			Status:  "request_form_issue",
			Message: err.Error(),
		}
	}

	return UploadFile(client, fD, body, writer, token)
}

// sends a request to OneDep to add multipart files to an existing deposition with id
func AddFileToDeposition(client *http.Client, deposition string, fileUpload FileUpload, file multipart.File, token string) (DepositionFile, *ResponseType) {

	fD, body, writer, err := prepareFileDeposition(deposition, fileUpload)
	if err != nil {
		return fD, &ResponseType{
			Status:  "request_body_issue",
			Message: err.Error(),
		}
	}
	// extract pixel spacing necessary to upload metadata
	for j := range NeedMeta {
		if fileUpload.Type == NeedMeta[j] {
			pixelSpacing, err := getMeta(file)
			if err != nil {
				log.Printf("failed to extract pixel spacing: %v; please provide it in OneDep manually!", err)
			}
			fD.PixelSpacing = pixelSpacing
		}
	}

	//upload file
	part, err := writer.CreateFormFile("file", fD.Name)
	if err != nil {
		return fD, &ResponseType{
			Status:  "request_file_issue",
			Message: err.Error(),
		}
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return fD, &ResponseType{
			Status:  "request_file_issue",
			Message: err.Error(),
		}
	}

	err = writer.Close()
	if err != nil {
		return fD, &ResponseType{
			Status:  "request_file_issue",
			Message: err.Error(),
		}
	}
	return UploadFile(client, fD, body, writer, token)
}

func UploadFile(client *http.Client, fD DepositionFile, body *bytes.Buffer, writer *multipart.Writer, token string) (DepositionFile, *ResponseType) {

	// Prepare the request
	url := baseURL + fD.DId + "/files/"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return fD, &ResponseType{
			Status:  "request_issue",
			Message: err.Error(),
		}
	}

	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return fD, &ResponseType{
			Status:  "request_error",
			Message: fmt.Sprintf("error sending request to the server: %v", err),
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		fD.Id, err = decodeFid(resp)

		if err != nil {
			return fD, &ResponseType{
				Status:  "decoding_error",
				Message: fmt.Sprintf("error decoding File ID: %v", err),
			}
		}
		return fD, nil
	} else {
		responseDecoded := decodeResponse(resp)
		return fD, &responseDecoded

	}
}

// sends a request to OneDep to add files to an existing deposition with id
func AddMetadataToFile(client *http.Client, fD DepositionFile, token string) (DepositionFile, *ResponseType) {

	// Prepare metadata request
	data := map[string]interface{}{
		"voxel": map[string]interface{}{
			"spacing": map[string]float32{
				"x": fD.PixelSpacing[0],
				"y": fD.PixelSpacing[1],
				"z": fD.PixelSpacing[2],
			},
			"contour": fD.ContourLevel, // There seems to be no way to extract it from header?
		},
		"description": fD.Details,
	}

	jsonBody, err := json.Marshal(data)
	if err != nil {
		return fD, &ResponseType{
			Status:  "JSON_error",
			Message: err.Error(),
		}
	}
	urlFileMeta := baseURL + fD.DId + "/files/" + fD.Id + "/metadata"
	req, err := http.NewRequest("POST", urlFileMeta, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fD, &ResponseType{
			Status:  "request_error",
			Message: err.Error(),
		}
	}

	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return fD, &ResponseType{
			Status:  "request_error",
			Message: fmt.Sprintf("error sending request to  to url %v: %v", urlFileMeta, err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		return fD, nil
	} else {
		responseDecoded := decodeResponse(resp)
		return fD, &responseDecoded
	}
}

// sends a request to OneDep to process a  deposition
func ProcessDeposition(client *http.Client, deposition string, token string) (string, error) {

	url := baseURL + deposition + "/process"
	req, _ := http.NewRequest("POST", url, new(bytes.Buffer))

	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("errored when sending request to the server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		return "success", nil
	} else {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("create: failed to create new deposition: status code %v, status %s, unreadable body", resp.StatusCode, resp.Status)
		}
		return "", fmt.Errorf("create: failed to create new deposition: status code %v, status %s, body %s", resp.StatusCode, resp.Status, string(body))
	}

}
