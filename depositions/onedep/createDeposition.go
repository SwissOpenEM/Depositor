package onedep

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
)

// decodes  the response
func decodeResponse[T FileResponse | DepositionResponse](resp *http.Response) (T, error) {
	var rOneDep T
	decoder := json.NewDecoder(resp.Body)
	err := decoder.Decode(&rOneDep)
	if err != nil {
		return rOneDep, err
	}
	return rOneDep, nil
}

// func decodeAnyResponse(resp *http.Response) (map[string]any, error) {
// 	var rOneDep map[string]any
// 	decoder := json.NewDecoder(resp.Body)
// 	err := decoder.Decode(&rOneDep)
// 	if err != nil {
// 		return rOneDep, err
// 	}
// 	return rOneDep, nil
// }

// sends a request to OneDep to create a new deposition
func CreateDeposition(client *http.Client, userInput UserInfo, token string) (DepositionResponse, *ResponseType) {
	var deposition DepositionResponse
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
	depositionResponse, err := decodeResponse[DepositionResponse](resp)
	if err != nil {
		return deposition, &ResponseType{
			Status:  "couldn't decode deposition response",
			Message: err.Error(),
		}
	}
	if resp.StatusCode == 200 || resp.StatusCode == 201 { // even not created instance, when e.g country is set wrong this seems to return a 200 with an error.
		return depositionResponse, nil
	} else {
		return deposition, &ResponseType{
			Status:  depositionResponse.Code,
			Message: depositionResponse.Message,
		}
	}
}

// creates a new instance of DepositionFile
func NewDepositionFile(depositionID string, fileUpload FileUpload) *DepositionFile {
	fD := DepositionFile{depositionID, 0, fileUpload.Name, fileUpload.Type, [3]float32{}, fileUpload.Contour, fileUpload.Details}
	return &fD
}

// reads the header of mrc files and extracts the pixel spacing, unpacks if was .gz file
func (fD *DepositionFile) getMeta(file multipart.File, gzipped bool) error {
	// https://bio3d.colorado.edu/imod/betaDoc/mrc_format.txt
	// words we need: Mode(4), sampling along axes of unit cell (8-10), cell dimensions in angstroms(11-13) --> pixel spacing = cell dim/sampling
	header := make([]byte, headerSize)
	var reader io.Reader

	if gzipped {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("failed to decompress: %v", err.Error())
		}
		defer gzReader.Close()
		reader = gzReader
	} else {
		reader = file
	}
	_, err := reader.Read(header)
	if err != nil {
		return fmt.Errorf("failed to read header: %v", err.Error())
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	var mode uint32 = binary.LittleEndian.Uint32(header[modeWord*wordSize : modeWord*wordSize+wordSize])
	var cellDim [3]float32
	if castFunc, ok := typeMap[mode]; ok {
		for i := 0; i < 3; i++ {
			cellDim[i] = castFunc(header[(cellDimWord+i)*wordSize : (cellDimWord+i)*wordSize+wordSize]).(float32)
			sampling := binary.LittleEndian.Uint32(header[(samplingWord+i)*wordSize : (samplingWord+i)*wordSize+wordSize])
			// Calculate pixel spacing
			fD.PixelSpacing[i] = cellDim[i] / float32(sampling)
		}
	} else {
		return fmt.Errorf("mode in the header is not described in EM community: %v", err)
	}
	return nil
}

// assigns specific values to DepositionFile, creates an instance for request body and multipart writer for file
func (fD *DepositionFile) PrepareDeposition() (*FileDepositionRequest, *ResponseType) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	fDreq := FileDepositionRequest{body, writer}

	err := fDreq.Writer.WriteField("name", fD.Name)
	if err != nil {
		return &fDreq, &ResponseType{
			Status:  "request_body_issue",
			Message: err.Error(),
		}
	}

	err = fDreq.Writer.WriteField("type", fD.Type)
	if err != nil {
		return &fDreq, &ResponseType{
			Status:  "request_body_issue",
			Message: err.Error(),
		}
	}
	return &fDreq, nil
}

// if this file is og "map" type, opens the header and extracts pixel spacing va;ue
func (fD *DepositionFile) ReadHeaderIfMap(file multipart.File, extension string) *ResponseType {
	var err error
	// extract pixel spacing necessary to upload metadata
	for j := range NeedMeta {
		if fD.Type == NeedMeta[j] {
			switch extension {
			case "gz":
				err = fD.getMeta(file, true)
			case "mrc":
				err = fD.getMeta(file, false)
			case "ccp4":
				err = fD.getMeta(file, false)
			default:
				log.Printf("failed to open header: %v extension is not implemented; please provide it in OneDep manually!", extension)
			}
			if err != nil {
				log.Printf("failed to extract pixel spacing: %v; please provide it in OneDep manually!", err)
			}
		}
	}
	return nil
}

// adds file to the multipart writer
func (fD *DepositionFile) AddFileToRequest(client *http.Client, file multipart.File, fDreq *FileDepositionRequest) (*FileDepositionRequest, *ResponseType) {
	//add file to the writer
	part, err := fDreq.Writer.CreateFormFile("file", fD.Name)
	if err != nil {
		return fDreq, &ResponseType{
			Status:  "request_file_issue",
			Message: err.Error(),
		}
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return fDreq, &ResponseType{
			Status:  "request_file_issue",
			Message: err.Error(),
		}
	}

	err = fDreq.Writer.Close()
	if err != nil {
		return fDreq, &ResponseType{
			Status:  "request_file_issue",
			Message: err.Error(),
		}
	}
	return fDreq, nil
}

// sends request to upload file to OneDep
func (fD *DepositionFile) UploadFile(client *http.Client, fDreq *FileDepositionRequest, token string) (FileResponse, *ResponseType) {
	var uploadedFile FileResponse
	// Prepare the request
	url := baseURL + fD.DId + "/files/"
	req, err := http.NewRequest("POST", url, fDreq.Body)
	if err != nil {
		return uploadedFile, &ResponseType{
			Status:  "request_issue",
			Message: err.Error(),
		}
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", fDreq.Writer.FormDataContentType())

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return uploadedFile, &ResponseType{
			Status:  "request_error",
			Message: fmt.Sprintf("error sending request to the server: %v", err),
		}
	}
	defer resp.Body.Close()
	decodedFileResponse, err := decodeResponse[FileResponse](resp)
	if err != nil {
		return uploadedFile, &ResponseType{
			Status:  "decoding_error",
			Message: fmt.Sprintf("error decoding File ID: %v", err),
		}
	}
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		fD.Id = decodedFileResponse.Id
		return decodedFileResponse, nil
	} else {
		return uploadedFile, &ResponseType{
			Status:  "failed_upload",
			Message: "some problem ", //decodedFileResponse.Errors,
		}
	}
}

// sends a request to OneDep to add files' metadata
func (fD *DepositionFile) AddMetadataToFile(client *http.Client, token string) (FileResponse, *ResponseType) {
	var uploadedFile FileResponse
	data := map[string]interface{}{
		"voxel": map[string]interface{}{
			"spacing": map[string]float32{
				"x": fD.PixelSpacing[0],
				"y": fD.PixelSpacing[1],
				"z": fD.PixelSpacing[2],
			},
			"contour": fD.ContourLevel,
		},
		"description": fD.Details, // Doesn't propagate in OneDep
	}
	jsonBody, err := json.Marshal(data)
	if err != nil {
		return uploadedFile, &ResponseType{
			Status:  "JSON_error",
			Message: err.Error(),
		}
	}
	urlFileMeta := baseURL + fD.DId + "/files/" + strconv.Itoa(fD.Id) + "/metadata"
	req, err := http.NewRequest("POST", urlFileMeta, bytes.NewBuffer(jsonBody))
	if err != nil {
		return uploadedFile, &ResponseType{
			Status:  "request_error",
			Message: err.Error(),
		}
	}

	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return uploadedFile, &ResponseType{
			Status:  "request_error",
			Message: fmt.Sprintf("error sending request to  to url %v: %v", urlFileMeta, err),
		}
	}
	defer resp.Body.Close()

	decodedFileResponse, err := decodeResponse[FileResponse](resp)
	if err != nil {
		return uploadedFile, &ResponseType{
			Status:  "decoding_error",
			Message: fmt.Sprintf("error decoding File ID: %v", err),
		}
	}
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		return decodedFileResponse, nil
	} else {
		return uploadedFile, &ResponseType{
			Status:  "failed_upload",
			Message: "some problem ", //decodedFileResponse.Errors,
		}
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
