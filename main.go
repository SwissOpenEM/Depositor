package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/SwissOpenEM/Depositor/depositions/onedep"

	docs "github.com/SwissOpenEM/Depositor/docs"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	parser "github.com/osc-em/converter-OSCEM-to-mmCIF/parser"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

//	@title			OpenEm Depositor API
//	@version		api/v1
//	@description	Rest API for communication between SciCat frontend and depositor backend. Backend service enables deposition of datasets to OneDep API.

var version string = "DEV"
var PORT string = getEnv("PORT", "8080")
var ALLOW_ORIGINS string = getEnv("ALLOW_ORIGINS", "http://localhost:4200")

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// deferClose schedules a file closer to be called, logging any errors
func deferClose(closer io.Closer, resourceName string) {
	if err := closer.Close(); err != nil {
		log.Printf("Warning: failed to close %s: %v", resourceName, err)
	}
}

// deferRemove schedules a file removal, logging any errors
func deferRemove(path string) {
	if err := os.Remove(path); err != nil {
		log.Printf("Warning: failed to remove temporary file %s: %v", path, err)
	}
}

// respondWithError sends a consistent error response to the client
func respondWithError(c *gin.Context, statusCode int, status, message string) {
	c.JSON(statusCode, onedep.ResponseType{
		Status:  status,
		Message: message,
	})
}

// respondBadRequest sends a 400 Bad Request with consistent formatting
func respondBadRequest(c *gin.Context, status, message string) {
	respondWithError(c, http.StatusBadRequest, status, message)
}

func IsValidJSON(str string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(str), &js) == nil
}

func IsGzipped(reader io.Reader) (bool, error) {
	bReader := bufio.NewReader(reader)
	testBytes, err := bReader.Peek(64) //read a few bytes without consuming
	if err != nil {
		return false, err
	}
	contentType := http.DetectContentType(testBytes)
	if strings.Contains(contentType, "x-gzip") {
		return true, nil
	} else {
		return false, nil
	}
}

// decompressGzipToFile decompresses a gzipped reader into the provided output file
// Returns an error if the input is not seekable, or if decompression fails
func decompressGzipToFile(input io.Reader, output *os.File) error {
	// Reset file pointer to beginning
	seeker, ok := input.(io.Seeker)
	if !ok {
		return fmt.Errorf("file does not support seeking")
	}

	if _, err := seeker.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to reset file pointer: %w", err)
	}

	// Create gzip reader
	gzipReader, err := gzip.NewReader(input)
	if err != nil {
		return fmt.Errorf("failed to open gzipped file: %w", err)
	}
	defer deferClose(gzipReader, "gzip reader")

	// Copy decompressed content to output file
	if _, err := io.Copy(output, gzipReader); err != nil {
		return fmt.Errorf("failed to copy decompressed content: %w", err)
	}

	return nil
}

// isPDBFile checks if a filename represents a PDB file (handles both .pdb and .pdb.gz)
func isPDBFile(filename string) bool {
	return strings.HasSuffix(filename, ".pdb") || strings.HasSuffix(filename, ".pdb.gz")
}

// Create handles the creation of a new deposition
// @Summary Create a new deposition to OneDep
// @Description Create a new deposition by uploading experiment and user details to OneDep API.
// @Tags onedep
// @Accept application/json
// @Produce json
// @Param	request	body	onedep.RequestCreate	true "User information"
// @Success 201 {object} onedep.DepositionResponse "Success response with Deposition ID"
// @Failure 400 {object} onedep.ResponseType "Error response"
// @Failure 500 {object} onedep.ResponseType "Internal server error"
// @Router /onedep [post]
func Create(c *gin.Context) {

	var body onedep.RequestCreate

	// Bind JSON payload to the struct
	if err := c.ShouldBindJSON(&body); err != nil {
		respondBadRequest(c, "invalid_request_body", err.Error())
		return
	}
	email := body.Email
	var experiments []onedep.EmMethod
	if exp, ok := onedep.EmMethods[body.Method]; ok {
		experiments = []onedep.EmMethod{exp}
		experiments[0].Coordinates = body.Coordinates
	} else {
		respondBadRequest(c, "method_unknown", fmt.Sprintf("unknown EM method %v", body.Method))
		return
	}
	bearer := body.JWTToken
	depData := onedep.UserInfo{
		Email:       email,
		Users:       body.OrcidIds,
		Country:     "United States", // body.country
		Experiments: experiments,
		Password:    body.Password,
	}

	client := &http.Client{}

	deposition, resp := onedep.CreateDeposition(client, depData, bearer)
	if resp != nil {
		respondBadRequest(c, resp.Status, resp.Message)
		return
	}
	c.JSON(http.StatusCreated, deposition)

}

// AddFiles handles adding files to an existing deposition. It also opens a header of mrc files and extracts the pixel spacing
// @Summary Add file, pixel spacing, contour level and description to deposition in OneDep
// @Description Uploading file, and metadata to OneDep API.
// @Tags onedep
// @Accept multipart/form-data
// @Produce json
// @Param depID path string true "Deposition ID to which a file should be uploaded"
// @Param file formData []file true "File to upload" collectionFormat(multi)
// @Param fileMetadata formData string true "File metadata as a JSON string"
// @Param jwtToken formData string true "JWT token for OneDep API"
// @Success 200 {object} onedep.FileResponse "File ID"
// @Failure 400 {object} onedep.ResponseType "Error response"
// @Failure 500 {object} onedep.ResponseType "Internal server error"
// @Router /onedep/{depID}/file [post]
func AddFile(c *gin.Context) {
	err := c.Request.ParseMultipartForm(10 << 20)
	if err != nil {
		respondBadRequest(c, "form_invalid", "Failed to parse Form data.")
		return
	}
	depID := c.Param("depID")
	bearer := c.PostForm("jwtToken")
	cooFile := c.Request.MultipartForm.File["file"][0] //file
	metadataFilesStr := c.PostForm("fileMetadata")     //files Metadata

	if metadataFilesStr == "{}" {
		respondBadRequest(c, "form_invalid", "Missing metadata for files information.")
		return
	}

	var fileMetadata onedep.FileUpload
	err = json.Unmarshal([]byte(metadataFilesStr), &fileMetadata)
	if err != nil {
		respondBadRequest(c, "JSON_invalid", fmt.Sprintf("error decoding JSON: %v", err.Error()))
		return
	}
	fileName := strings.Split(cooFile.Filename, ".")
	extension := fileName[len(fileName)-1]

	file, err := cooFile.Open()
	if err != nil {
		respondBadRequest(c, "file_header_invalid", fmt.Sprintf("failed to open file header: %v", err.Error()))
		return
	}

	client := &http.Client{}

	fD := onedep.NewDepositionFile(depID, fileMetadata)

	fDReq, errResp := fD.PrepareDeposition()
	if errResp != nil {
		c.JSON(http.StatusBadRequest, errResp)
		return
	}

	errResp = fD.ReadHeaderIfMap(file, extension)
	if errResp != nil {
		c.JSON(http.StatusBadRequest, errResp)
		return
	}
	fDReq, errResp = fD.AddFileToRequest(client, file, fDReq)
	if errResp != nil {
		c.JSON(http.StatusBadRequest, errResp)
		return
	}
	defer deferClose(file, "uploaded file")
	uploadedFileDecoded, errResp := fD.UploadFile(client, fDReq, bearer)
	if errResp != nil {
		c.JSON(http.StatusBadRequest, errResp)
		return
	}

	for j := range onedep.NeedMeta {
		if string(fileMetadata.Type) == onedep.NeedMeta[j] {
			uploadedFileDecoded, errResp = fD.AddMetadataToFile(client, bearer)
			if errResp != nil {
				c.JSON(http.StatusBadRequest, errResp)
				return
			}
			c.JSON(http.StatusOK, uploadedFileDecoded)
			return
		}
	}
	c.JSON(http.StatusOK, uploadedFileDecoded)
}

// AddMetadata handles adding metadata to an existing deposition.
// @Summary Add a cif file with metadata to deposition in OneDep
// @Description Uploading metadata file to OneDep API. This is created by parsing the JSON metadata into the converter.
// @Tags onedep
// @Accept multipart/form-data
// @Produce json
// @Param depID path string true "Deposition ID to which a file should be uploaded"
// @Param jwtToken formData string true "JWT token for OneDep API"
// @Param scientificMetadata formData string true "Scientific metadata as a JSON string; expects elements from OSCEM on the top level"
// @Success 200 {object} onedep.FileResponse "File ID"
// @Failure 400 {object} onedep.ResponseType "Error response"
// @Failure 500 {object} onedep.ResponseType "Internal server error"
// @Router /onedep/{depID}/metadata [post]
func AddMetadata(c *gin.Context) {
	err := c.Request.ParseMultipartForm(10 << 20)
	if err != nil {
		respondBadRequest(c, "body_invalid", "Failed to parse Form data.")
		return
	}

	depID := c.Param("depID")
	bearer := c.PostForm("jwtToken")

	// FIX ME add an OSCEM SCHEMA
	// Extract entries from multipart form
	metadataStr := c.PostForm("scientificMetadata")
	if metadataStr == "{}" {
		respondBadRequest(c, "body_invalid", "Missing scientific metadata.")
		return
	}
	// Parse  JSON string into the Metadata
	var scientificMetadata map[string]any
	err = json.Unmarshal([]byte(metadataStr), &scientificMetadata)
	if err != nil {
		respondBadRequest(c, "body_invalid", "Failed to parse scientific metadata to JSON.")
		return
	}
	// create a temporary cif file that will be used for a deposition
	finalCif, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		respondBadRequest(c, "cif_creation_fails", "Failed to create a file to write cif file with metadata.")
		return
	}
	finalCifPath := finalCif.Name()
	defer deferRemove(finalCifPath)

	// convert OSCEM-JSON to mmCIF
	cifText, err := parser.EMDBconvert(
		scientificMetadata,
		"",
		"data/conversions.csv",
		"data/mmcif_pdbx_v50.dic",
	)
	if err != nil {
		respondBadRequest(c, "conversion_to_cif_fails", err.Error())
		return
	}
	err = parser.WriteCif(cifText, finalCifPath)
	if err != nil {
		respondBadRequest(c, "writing_cif_fails", err.Error())
		return
	}
	client := &http.Client{}
	metadataFile := onedep.FileUpload{
		Name: "metadata.cif",
		Type: onedep.MD_CIF,
	}
	fmt.Println("will transfer md cif!")
	cifFile, err := os.Open(finalCifPath)
	if err != nil {
		respondBadRequest(c, "cif_file_issue", err.Error())
		return
	}
	defer deferClose(cifFile, "metadata cif file")

	fD := onedep.NewDepositionFile(depID, metadataFile)

	fDReq, errResp := fD.PrepareDeposition()
	if errResp != nil {
		c.JSON(http.StatusBadRequest, errResp)
		return
	}

	fDReq, errResp = fD.AddFileToRequest(client, cifFile, fDReq)
	if errResp != nil {
		c.JSON(http.StatusBadRequest, errResp)
		return
	}

	uploadedFile, errResp := fD.UploadFile(client, fDReq, bearer)
	if errResp != nil {
		c.JSON(http.StatusBadRequest, errResp)
		return
	}

	c.JSON(http.StatusOK, uploadedFile)
}

// AddCoordinates handles adding coordinates files to an existing deposition.
// @Summary Add coordinates and description to deposition in OneDep
// @Description Uploading file to OneDep API.
// @Tags onedep
// @Accept multipart/form-data
// @Produce json
// @Param depID path string true "Deposition ID to which a file should be uploaded"
// @Param jwtToken formData string true "JWT token for OneDep API"
// @Param file formData file true "File to upload"
// @Param scientificMetadata formData string true "Scientific metadata as a JSON string; expects elements from OSCEM on the top level"
// @Success 200 {object} onedep.FileResponse "File ID"
// @Failure 400 {object} onedep.ResponseType "Error response"
// @Failure 500 {object} onedep.ResponseType "Internal server error"
// @Router /onedep/{depID}/pdb [post]
func AddCoordinates(c *gin.Context) {
	err := c.Request.ParseMultipartForm(10 << 20)
	if err != nil {
		respondBadRequest(c, "body_invalid", "Failed to parse Form data.")
		return
	}

	depID := c.Param("depID")
	bearer := c.PostForm("jwtToken")
	cooFile := c.Request.MultipartForm.File["file"][0]

	// parse extracted metadata from SciCat
	metadataStr := c.PostForm("scientificMetadata")
	if metadataStr == "{}" {
		respondBadRequest(c, "body_invalid", "Missing scientific metadata.")
		return
	}
	// Parse  JSON string into the OSCEM format
	var scientificMetadata map[string]any
	err = json.Unmarshal([]byte(metadataStr), &scientificMetadata)
	if err != nil {
		respondBadRequest(c, "body_invalid", "Failed to parse scientific metadata to JSON.")
		return
	}
	// open the multipart file header
	fMP, err := cooFile.Open()
	if err != nil {
		respondBadRequest(c, "file_invalid", fmt.Sprintf("Failed to open file header: %v", err.Error()))
		return
	}

	passedName := cooFile.Filename
	isPDB := isPDBFile(passedName)
	f := io.Reader(fMP)
	defer deferClose(fMP, "multipart file")

	fConv, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		respondBadRequest(c, "file_invalid", fmt.Sprintf("failed to create cif file to combine metadata and coordinates: %v", err.Error()))
		return
	}

	cifFileForConverterPath := fConv.Name()
	defer deferRemove(cifFileForConverterPath)

	gzipped, err := IsGzipped(f)
	if err != nil {
		respondBadRequest(c, "file_invalid", fmt.Sprintf("failed to open cif file to read coordinates: %v", err.Error()))
		return
	}
	if gzipped {
		if err := decompressGzipToFile(f, fConv); err != nil {
			respondBadRequest(c, "file_invalid", fmt.Sprintf("failed to decompress file: %v", err.Error()))
			return
		}
		f = fConv
	}
	if isPDB {
		//need to save file locally first
		if !gzipped {
			_, err := io.Copy(fConv, f)
			if err != nil {
				respondBadRequest(c, "file_invalid", fmt.Sprintf("failed to copy coordinates to temporary cif file: %v", err.Error()))
				return
			}
		}
		pythonExecCh := make(chan struct {
			errorMsg string
			err      error
		})

		go func() {
			cPy := exec.Command("scripts/bin/pdb_extract.py", "-EM", "-iPDB", cifFileForConverterPath, "-o", cifFileForConverterPath)

			var outputBuffer bytes.Buffer
			cPy.Stdout = &outputBuffer
			cPy.Stderr = &outputBuffer

			err := cPy.Run()

			if err != nil {
				pythonExecCh <- struct {
					errorMsg string
					err      error
				}{outputBuffer.String(), err}
				return

			}

			pythonExecCh <- struct {
				errorMsg string
				err      error
			}{outputBuffer.String(), nil}
		}()

		// Wait for processing to finish
		pyExec := <-pythonExecCh
		if pyExec.err != nil {
			respondBadRequest(c, pyExec.err.Error(), pyExec.errorMsg)
			return
		}
		// read converted file again
		f, err = os.Open(cifFileForConverterPath)
		if err != nil {
			respondBadRequest(c, "pdb_invalid_2", fmt.Sprintf("pdb_extract conversion from pdb to cif failed to write a file: %v", err.Error()))
			return
		}
	}
	defer deferClose(fConv, "temporary converter file")
	cifText, err := parser.PDBconvertFromReader(
		scientificMetadata,
		"",
		"data/conversions.csv",
		"data/mmcif_pdbx_v50.dic",
		f, // changed the oscem converter
	)
	if err != nil {
		respondBadRequest(c, "conversion_to_cif_fails", err.Error())
		return
	}
	// defer os.Remove(fileUncompressed)

	err = parser.WriteCif(cifText, cifFileForConverterPath)
	if err != nil {
		respondBadRequest(c, "writing_cif_fails", err.Error())
		return
	}
	client := &http.Client{}
	mockFileUpload := onedep.FileUpload{
		Name:    "coordinates.cif",
		Type:    onedep.CO_CIF,
		Details: "",
	}
	cifFile, err := os.Open(cifFileForConverterPath)
	if err != nil {
		respondBadRequest(c, "cif_file_issue", err.Error())
		return
	}
	defer deferClose(cifFile, "coordinates cif file")

	fD := onedep.NewDepositionFile(depID, mockFileUpload)
	fDReq, errResp := fD.PrepareDeposition()
	if errResp != nil {
		c.JSON(http.StatusBadRequest, errResp)
		return
	}
	fDReq, errResp = fD.AddFileToRequest(client, cifFile, fDReq)
	if errResp != nil {
		c.JSON(http.StatusBadRequest, errResp)
		return
	}

	uploadedFile, errResp := fD.UploadFile(client, fDReq, bearer)
	if errResp != nil {
		c.JSON(http.StatusBadRequest, errResp)
		return
	}

	c.JSON(http.StatusOK, uploadedFile)

}

// DownloadMetadata handles getting a cif file with metadata.
// @Summary Get a cif file with metadata for manual deposition in OneDep
// @Description Downloading a metadata file. Invokes converter and starts download.
// @Tags onedep
// @Accept application/json
// @Produce application/octet-stream
// @Param scientificMetadata body object true "Scientific metadata as a JSON string; expects elements from OSCEM on the top level"
// @Success 200 {file} application/octet-stream "File download for the generated metadata.cif"
// @Failure 400 {object} onedep.ResponseType "Error response"
// @Failure 500 {object} onedep.ResponseType "Internal server error"
// @Router /onedep/metadata [post]
func DownloadMetadata(c *gin.Context) {
	var scientificMetadata map[string]interface{}

	// Bind the JSON payload into a metadata
	if err := c.ShouldBindJSON(&scientificMetadata); err != nil {
		respondBadRequest(c, "invalid_request_body", fmt.Sprintf("Failed to parse metadata body: %v", err.Error()))
		return
	}
	// create a temporary cif file that will be used for a deposition
	finalCif, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		respondBadRequest(c, "cif_creation_fails", "Failed to create a file to write cif file with metadata.")
		return
	}
	finalCifPath := finalCif.Name()
	defer deferRemove(finalCifPath)

	// convert OSCEM-JSON to mmCIF
	cifText, err := parser.EMDBconvert(
		scientificMetadata,
		"",
		"data/conversions.csv",
		"data/mmcif_pdbx_v50.dic",
	)
	if err != nil {
		respondBadRequest(c, "conversion_to_cif_fails", err.Error())
		return
	}
	err = parser.WriteCif(cifText, finalCifPath)
	if err != nil {
		respondBadRequest(c, "writing_cif_fails", err.Error())
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.FileAttachment(finalCifPath, "metadata.cif")

}

// DownloadCoordinates handles parsing an existing cif and metadata and downloading a new cif file.
// @Summary Get a cif file with metadata and coordinates for manual deposition in OneDep
// @Description Downloading a metadata file. Invokes converter and starts download.
// @Tags onedep
// @Accept multipart/form-data
// @Produce application/octet-stream
// @Param scientificMetadata formData string true "Scientific metadata as a JSON string; expects elements from OSCEM on the top level"
// @Param file formData file true "File to upload"
// @Success 200 {file} application/octet-stream "File download for the generated metadata.cif"
// @Failure 400 {object} onedep.ResponseType "Error response"
// @Failure 500 {object} onedep.ResponseType "Internal server error"
// @Router /onedep/pdb [post]
func DownloadCoordinatesWithMetadata(c *gin.Context) {

	err := c.Request.ParseMultipartForm(10 << 20)
	if err != nil {
		respondBadRequest(c, "body_invalid", "Failed to parse Form data.")
		return
	}

	cooFile := c.Request.MultipartForm.File["file"][0]

	// parse extracted metadata from SciCat
	metadataStr := c.PostForm("scientificMetadata")
	if metadataStr == "{}" {
		respondBadRequest(c, "body_invalid", "Missing scientific metadata.")
		return
	}
	// Parse  JSON string into the OSCEM format
	var scientificMetadata map[string]any
	err = json.Unmarshal([]byte(metadataStr), &scientificMetadata)
	if err != nil {
		respondBadRequest(c, "body_invalid", "Failed to parse scientific metadata to JSON.")
		return
	}
	// open the multipart file header
	fMP, err := cooFile.Open()
	if err != nil {
		respondBadRequest(c, "file_invalid", fmt.Sprintf("Failed to open file header: %v", err.Error()))
		return
	}

	passedName := cooFile.Filename
	isPDB := isPDBFile(passedName)
	f := io.Reader(fMP)
	defer deferClose(fMP, "multipart file")

	fConv, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		respondBadRequest(c, "file_invalid", fmt.Sprintf("failed to create cif file to combine metadata and coordinates: %v", err.Error()))
		return
	}
	cifFileForConverterPath := fConv.Name()
	defer deferRemove(cifFileForConverterPath)

	gzipped, err := IsGzipped(f)
	if err != nil {
		respondBadRequest(c, "file_invalid", fmt.Sprintf("failed to open cif file to read coordinates: %v", err.Error()))
		return
	}
	if gzipped {
		if err := decompressGzipToFile(f, fConv); err != nil {
			respondBadRequest(c, "file_invalid", fmt.Sprintf("failed to decompress file: %v", err.Error()))
			return
		}
	}

	defer deferClose(fConv, "temporary converter file")

	if isPDB {
		//need to save file locally first
		if !gzipped {
			_, err := io.Copy(fConv, f)
			if err != nil {
				respondBadRequest(c, "file_invalid", fmt.Sprintf("failed to copy coordinates to temporary cif file: %v", err.Error()))
				return
			}
		}
		pythonExecCh := make(chan struct {
			errorMsg string
			err      error
		})

		go func() {
			cPy := exec.Command("scripts/bin/pdb_extract.py", "-EM", "-iPDB", cifFileForConverterPath, "-o", cifFileForConverterPath)

			var outputBuffer bytes.Buffer
			cPy.Stdout = &outputBuffer
			cPy.Stderr = &outputBuffer

			err := cPy.Run()

			if err != nil {
				pythonExecCh <- struct {
					errorMsg string
					err      error
				}{outputBuffer.String(), err}
				return

			}

			pythonExecCh <- struct {
				errorMsg string
				err      error
			}{outputBuffer.String(), nil}

		}()

		// Wait for processing to finish
		pyExec := <-pythonExecCh
		if pyExec.err != nil {
			respondBadRequest(c, pyExec.err.Error(), pyExec.errorMsg)
			return
		}
		// read converted file again
		f, err = os.Open(cifFileForConverterPath)
		if err != nil {
			respondBadRequest(c, "pdb_invalid_2", fmt.Sprintf("pdb_extract conversion from pdb to cif failed to write a file: %v", err.Error()))
			return
		}
	}

	cifText, err := parser.PDBconvertFromReader(
		scientificMetadata,
		"",
		"data/conversions.csv",
		"data/mmcif_pdbx_v50.dic",
		f,
	)
	if err != nil {
		respondBadRequest(c, "conversion_to_cif_fails", err.Error())
		return
	}
	// defer os.Remove(fileUncompressed)

	err = parser.WriteCif(cifText, cifFileForConverterPath)
	if err != nil {
		respondBadRequest(c, "writing_cif_fails", err.Error())
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.FileAttachment(cifFileForConverterPath, "metadata.cif")
}

// Create handles the creation of a new deposition
// @Summary Process deposition to OneDep
// @Description Process a deposition in OneDep API.
// @Tags onedep
// @Accept application/json
// @Produce json
// @Param depID path string true "Deposition ID to which a file should be uploaded"
// @Param jwtToken  formData string true "JWT token for OneDep API"
// @Success 200 {object} onedep.CreatedDeposition "Deposition ID"
// @Failure 400 {object} onedep.ResponseType "Error response"
// @Failure 500 {object} onedep.ResponseType "Internal server error"
// @Router /onedep/{depID}/process [post]
func StartProcess(c *gin.Context) {
	depID := c.Param("depID")
	client := &http.Client{}
	bearer := c.PostForm("jwtToken")
	_, err := onedep.ProcessDeposition(client, depID, bearer)
	if err != nil {
		respondBadRequest(c, "process_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"fileID": depID})
}

// EmpiarMetadata handles creating of json file according to schema at https://github.com/emdb-empiar/empiar-depositor/blob/master/empiar_depositor/empiar_deposition.schema.json
// @Summary creates a json file with metadata for deposition to EMPIAR
// @Description NA
// @Tags empiar
// @Accept application/json
// @Produce json
// @Param scientificMetadata body object true "Scientific metadata as a JSON string; expects elements from OSCEM on the top level"
// @Success 200 {object} empiar.Imageset "json file with metadata"
// @Failure 400 {object} onedep.ResponseType "Error response"
// @Failure 500 {object} onedep.ResponseType "Internal server error"
// @Router /empiar/metadata [post]
func EmpiarMetadata(c *gin.Context) {
	var scientificMetadata map[string]interface{}

	// Bind the JSON payload into a metadata
	if err := c.ShouldBindJSON(&scientificMetadata); err != nil {
		respondBadRequest(c, "invalid_request_body", fmt.Sprintf("Failed to parse metadata body: %v", err.Error()))
		return
	}
	// create a temporary cif file that will be used for a deposition
	finalCif, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		respondBadRequest(c, "cif_creation_fails", "Failed to create a file to write cif file with metadata.")
		return
	}
	finalCifPath := finalCif.Name()
	defer deferRemove(finalCifPath)

	// convert OSCEM-JSON to mmCIF
	cifText, err := parser.EMDBconvert(
		scientificMetadata,
		"",
		"data/conversions.csv",
		"data/mmcif_pdbx_v50.dic",
	)
	if err != nil {
		respondBadRequest(c, "conversion_to_cif_fails", err.Error())
		return
	}
	err = parser.WriteCif(cifText, finalCifPath)
	if err != nil {
		respondBadRequest(c, "writing_cif_fails", err.Error())
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.FileAttachment(finalCifPath, "metadata.cif")

}

// EmpiarSchema handles creating of json file according to schema at https://github.com/emdb-empiar/empiar-depositor/blob/master/empiar_depositor/empiar_deposition.schema.json
// @Summary creates a json file with metadata for deposition to EMPIAR
// @Description NA
// @Tags empiar
// @Accept application/json
// @Produce json
// @Success 200 {object} string "base64 encoded schema"
// @Failure 400 {object} onedep.ResponseType "Error response"
// @Failure 500 {object} onedep.ResponseType "Internal server error"
// @Router /empiar/schema [get]
func EmpiarSchema(c *gin.Context) {
	schema, err := os.ReadFile("data/empiar_deposition.schema.json")
	if err != nil {
		respondBadRequest(c, "no_empiar_schema", err.Error())
		return
	}
	if !IsValidJSON(string(schema)) {
		respondBadRequest(c, "schema_invalid", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"schema": b64.StdEncoding.EncodeToString(schema),
	})
}

//	returns the current version of the depositor
//
// @Summary Return current version
// @Description Create a new deposition by uploading experiments, files, and metadata to OneDep API.
// @Tags version
// @Produce json
// @Success 200 {string} string "Depositior version"
// @Failure 400 {object} string "Error response"
// @Router /version [get]
func GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": version})
}

func main() {
	fmt.Println(ALLOW_ORIGINS, PORT)
	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{ALLOW_ORIGINS},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
	}))

	docs.SwaggerInfo.BasePath = router.BasePath()
	router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	router.GET("/version", GetVersion)
	router.POST("/onedep", Create)
	router.POST("/onedep/:depID/file", AddFile)
	router.POST("/onedep/:depID/metadata", AddMetadata)
	router.POST("/onedep/:depID/pdb", AddCoordinates)
	router.POST("/onedep/:depID/process", StartProcess)
	router.POST("/onedep/metadata", DownloadMetadata)
	router.POST("/onedep/pdb", DownloadCoordinatesWithMetadata)

	router.POST("/empiar/metadata", EmpiarMetadata)
	router.GET("/empiar/schema", EmpiarSchema)

	if err := router.Run(":" + PORT); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		os.Exit(1)
	}
}
