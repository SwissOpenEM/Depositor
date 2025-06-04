package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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
var ALLOW_ORIGINS string = getEnv("ALLOW_ORIGINS", "http://localhost:4201")

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// Convert multipart.File to *os.File by saving it to a temporary file
func convertMultipartFileToFile(file multipart.File) (*os.File, error) {
	tempFile, err := os.CreateTemp("", "uploaded-*")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Copy the contents of the multipart.File to the temporary file
	if _, err := io.Copy(tempFile, file); err != nil {
		tempFile.Close()
		return nil, err
	}

	// Close and reopen the file to reset the read pointer for future operations
	if err := tempFile.Close(); err != nil {
		return nil, err
	}
	return os.Open(tempFile.Name())
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

		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "invalid_request_body",
			"message": err.Error(),
		})
		return
	}

	email := body.Email
	var experiments []onedep.EmMethod
	if exp, ok := onedep.EmMethods[body.Method]; ok {
		experiments = []onedep.EmMethod{exp}
	} else {

		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "method_unknown",
			"message": fmt.Sprintf("unknown EM method %v", body.Method),
		})
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
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  resp.Status,
			"message": resp.Message,
		})
		return
	} else {
		c.JSON(http.StatusCreated, deposition)
		return
	}

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
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "form_invalid",
			"message": "Failed to parse Form data.",
		})
		return
	}
	depID := c.Param("depID")
	bearer := c.PostForm("jwtToken")
	cooFile := c.Request.MultipartForm.File["file"][0] //file
	metadataFilesStr := c.PostForm("fileMetadata")     //files Metadata

	if metadataFilesStr == "{}" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "form_invalid",
			"message": "Missing metadata for files information.",
		})
		return
	}

	var fileMetadata onedep.FileUpload
	err = json.Unmarshal([]byte(metadataFilesStr), &fileMetadata)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "JSON_invalid",
			"message": fmt.Sprintf("error decoding JSON: %v", err.Error()),
		})
		return
	}
	fileName := strings.Split(cooFile.Filename, ".")
	extension := fileName[len(fileName)-1]

	file, err := cooFile.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "file_header_invalid",
			"message": fmt.Sprintf("failed to open file header: %v", err.Error()),
		})
		return
	}
	defer file.Close()

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
	uploadedFileDecoded, errResp := fD.UploadFile(client, fDReq, bearer)
	if errResp != nil {
		c.JSON(http.StatusBadRequest, errResp)
		return
	}

	for j := range onedep.NeedMeta {
		if fileMetadata.Type == onedep.NeedMeta[j] {
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
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "body_invalid",
			"message": "Failed to parse Form data.",
		})
		return
	}

	depID := c.Param("depID")
	bearer := c.PostForm("jwtToken")

	// FIX ME add an OSCEM SCHEMA
	// Extract entries from multipart form
	metadataStr := c.PostForm("scientificMetadata")
	if metadataStr == "{}" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "body_invalid",
			"message": "Missing scientific metadata.",
		})
		return
	}
	// Parse  JSON string into the Metadata
	var scientificMetadata map[string]any
	err = json.Unmarshal([]byte(metadataStr), &scientificMetadata)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "body_invalid",
			"message": "Failed to parse scientific metadata to JSON.",
		})
		return
	}
	// create a temporary cif file that will be used for a deposition
	finalCif, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "cif_creation_fails",
			"message": "Failed to create a file to write cif file with metadata.",
		})
		return
	}
	finalCifPath := finalCif.Name()

	// convert OSCEM-JSON to mmCIF
	cifText, err := parser.EMDBconvert(
		scientificMetadata,
		"",
		"data/conversions.csv",
		"data/mmcif_pdbx_v50.dic",
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "conversion_to_cif_fails",
			"message": err.Error(),
		})
		return
	}
	err = parser.WriteCif(cifText, finalCifPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "writing_cif_fails",
			"message": err.Error(),
		})
		return
	}
	client := &http.Client{}
	metadataFile := onedep.FileUpload{
		Name: "metadata.cif",
		Type: "co-cif", // FIX ME add appropriate type once it's implemented in OneDep API
	}
	cifFile, err := os.Open(finalCifPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, &onedep.ResponseType{
			Status:  "cif_file_issue",
			Message: err.Error(),
		})
	}
	defer cifFile.Close()

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
	defer os.Remove(finalCifPath)

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
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "body_invalid",
			"message": "Failed to parse Form data.",
		})
		return
	}

	depID := c.Param("depID")
	bearer := c.PostForm("jwtToken")
	cooFile := c.Request.MultipartForm.File["file"][0]

	// parse extracted metadata from SciCat
	metadataStr := c.PostForm("scientificMetadata")
	if metadataStr == "{}" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "body_invalid",
			"message": "Missing scientific metadata.",
		})
		return
	}
	// Parse  JSON string into the OSCEM format
	var scientificMetadata map[string]any
	err = json.Unmarshal([]byte(metadataStr), &scientificMetadata)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "body_invalid",
			"message": "Failed to parse scientific metadata to JSON.",
		})
		return
	}
	// open the multipart file header
	fMP, err := cooFile.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "file_invalid",
			"message": fmt.Sprintf("Failed to open file header: %v", err.Error()),
		})
		return
	}
	defer fMP.Close()

	passedName := cooFile.Filename
	var isPDB = false
	passedNameChunks := strings.Split(passedName, ".")
	for i := range passedNameChunks {
		if passedNameChunks[i] == "pdb" {
			isPDB = true
		}
	}
	f := io.Reader(fMP)
	fConv, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "file_invalid",
			"message": fmt.Sprintf("failed to create cif file to combine metadata and coordinates: %v", err.Error()),
		})
		return
	}

	cifFileForConverterPath := fConv.Name()
	defer fConv.Close()

	gzipped, err := IsGzipped(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "file_invalid",
			"message": fmt.Sprintf("failed to open cif file to read coordinates: %v", err.Error()),
		})
		return
	}
	if gzipped {
		seeker, ok := f.(io.Seeker)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "file_invalid",
				"message": "file does not support seeking",
			})
			return
		}

		_, err := seeker.Seek(0, io.SeekStart)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "file_invalid",
				"message": fmt.Sprintf("failed to reset file pointer: %v", err.Error()),
			})
			return
		}

		gzipReader, err := gzip.NewReader(f)
		if err != nil {

			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "file_invalid",
				"message": fmt.Sprintf("failed to open gzipped cif file : %v", err.Error()),
			})
			return
		}
		defer gzipReader.Close()

		_, err = io.Copy(fConv, gzipReader)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "file_invalid",
				"message": fmt.Sprintf("failed to copy untared coordinates to temporary cif file: %v", err.Error()),
			})
			return
		}

		f = fConv
	}
	if isPDB {
		//need to save file locally first
		if !gzipped {
			_, err := io.Copy(fConv, f)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"status":  "file_invalid",
					"message": fmt.Sprintf("failed to copy coordinates to temporary cif file: %v", err.Error()),
				})
				return
			}
			fConv.Close()
			f = fConv
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

			return
		}()

		// Wait for processing to finish
		pyExec := <-pythonExecCh
		if pyExec.err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  pyExec.err.Error(),
				"message": pyExec.errorMsg,
			})
			return
		}
		// read converted file again
		f, err = os.Open(cifFileForConverterPath)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "pdb_invalid_2",
				"message": fmt.Sprintf("pdb_extract conversion from pdb to cif failed to write a file: %v", err.Error()),
			})
		}
	}
	cifText, err := parser.PDBconvertFromReader(
		scientificMetadata,
		"",
		"data/conversions.csv",
		"data/mmcif_pdbx_v50.dic",
		f, // changed the oscem converter
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "conversion_to_cif_fails",
			"message": err.Error(),
		})
		return
	}
	// defer os.Remove(fileUncompressed)

	err = parser.WriteCif(cifText, cifFileForConverterPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "writing_cif_fails",
			"message": err.Error(),
		})
		return
	}
	client := &http.Client{}
	mockFileUpload := onedep.FileUpload{
		Name:    "coordinates.cif",
		Type:    "co-cif",
		Details: "",
	}
	cifFile, err := os.Open(cifFileForConverterPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, &onedep.ResponseType{
			Status:  "cif_file_issue",
			Message: err.Error(),
		})
	}
	defer cifFile.Close()

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
	//defer os.Remove(cifFileForConverterPath)

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
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "invalid_request_body",
			"message": fmt.Sprintf("Failed to parse metadata body: %v", err.Error()),
		})
		return
	}
	// create a temporary cif file that will be used for a deposition
	finalCif, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "cif_creation_fails",
			"message": "Failed to create a file to write cif file with metadata.",
		})
		return
	}
	finalCifPath := finalCif.Name()

	// convert OSCEM-JSON to mmCIF
	cifText, err := parser.EMDBconvert(
		scientificMetadata,
		"",
		"data/conversions.csv",
		"data/mmcif_pdbx_v50.dic",
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "conversion_to_cif_fails",
			"message": err.Error(),
		})
		return
	}
	err = parser.WriteCif(cifText, finalCifPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "writing_cif_fails",
			"message": err.Error(),
		})
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.FileAttachment(finalCifPath, "metadata.cif")

	defer os.Remove(finalCifPath)

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
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "body_invalid",
			"message": "Failed to parse Form data.",
		})
		return
	}

	cooFile := c.Request.MultipartForm.File["file"][0]

	// parse extracted metadata from SciCat
	metadataStr := c.PostForm("scientificMetadata")
	if metadataStr == "{}" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "body_invalid",
			"message": "Missing scientific metadata.",
		})
		return
	}
	// Parse  JSON string into the OSCEM format
	var scientificMetadata map[string]any
	err = json.Unmarshal([]byte(metadataStr), &scientificMetadata)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "body_invalid",
			"message": "Failed to parse scientific metadata to JSON.",
		})
		return
	}
	// open the multipart file header
	fMP, err := cooFile.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "file_invalid",
			"message": fmt.Sprintf("Failed to open file header: %v", err.Error()),
		})
		return
	}
	defer fMP.Close()

	passedName := cooFile.Filename
	var isPDB = false
	passedNameChunks := strings.Split(passedName, ".")
	for i := range passedNameChunks {
		if passedNameChunks[i] == "pdb" {
			isPDB = true
		}
	}
	f := io.Reader(fMP)
	fConv, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "file_invalid",
			"message": fmt.Sprintf("failed to create cif file to combine metadata and coordinates: %v", err.Error()),
		})
		return
	}

	cifFileForConverterPath := fConv.Name()
	defer fConv.Close()

	gzipped, err := IsGzipped(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "file_invalid",
			"message": fmt.Sprintf("failed to open cif file to read coordinates: %v", err.Error()),
		})
		return
	}
	if gzipped {
		seeker, ok := f.(io.Seeker)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "file_invalid",
				"message": "file does not support seeking",
			})
			return
		}

		_, err := seeker.Seek(0, io.SeekStart)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "file_invalid",
				"message": fmt.Sprintf("failed to reset file pointer: %v", err.Error()),
			})
			return
		}

		gzipReader, err := gzip.NewReader(f)
		if err != nil {

			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "file_invalid",
				"message": fmt.Sprintf("failed to open gzipped cif file : %v", err.Error()),
			})
			return
		}
		defer gzipReader.Close()

		_, err = io.Copy(fConv, gzipReader)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "file_invalid",
				"message": fmt.Sprintf("failed to copy untared coordinates to temporary cif file: %v", err.Error()),
			})
			return
		}

		f = fConv
	}
	if isPDB {
		//need to save file locally first
		if !gzipped {
			_, err := io.Copy(fConv, f)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"status":  "file_invalid",
					"message": fmt.Sprintf("failed to copy coordinates to temporary cif file: %v", err.Error()),
				})
				return
			}
			fConv.Close()
			f = fConv
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

			return
		}()

		// Wait for processing to finish
		pyExec := <-pythonExecCh
		if pyExec.err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  pyExec.err.Error(),
				"message": pyExec.errorMsg,
			})
			return
		}
		// read converted file again
		f, err = os.Open(cifFileForConverterPath)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "pdb_invalid_2",
				"message": fmt.Sprintf("pdb_extract conversion from pdb to cif failed to write a file: %v", err.Error()),
			})
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
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "conversion_to_cif_fails",
			"message": err.Error(),
		})
		return
	}
	// defer os.Remove(fileUncompressed)

	err = parser.WriteCif(cifText, cifFileForConverterPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "writing_cif_fails",
			"message": err.Error(),
		})
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.FileAttachment(cifFileForConverterPath, "metadata.cif")
	defer os.Remove(cifFileForConverterPath)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err})
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
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "invalid_request_body",
			"message": fmt.Sprintf("Failed to parse metadata body: %v", err.Error()),
		})
		return
	}
	// create a temporary cif file that will be used for a deposition
	finalCif, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "cif_creation_fails",
			"message": "Failed to create a file to write cif file with metadata.",
		})
		return
	}
	finalCifPath := finalCif.Name()

	// convert OSCEM-JSON to mmCIF
	cifText, err := parser.EMDBconvert(
		scientificMetadata,
		"",
		"data/conversions.csv",
		"data/mmcif_pdbx_v50.dic",
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "conversion_to_cif_fails",
			"message": err.Error(),
		})
		return
	}
	err = parser.WriteCif(cifText, finalCifPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "writing_cif_fails",
			"message": err.Error(),
		})
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.FileAttachment(finalCifPath, "metadata.cif")

	defer os.Remove(finalCifPath)

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
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "no_empiar_schema",
			"message": err.Error(),
		})
		return
	}
	if !IsValidJSON(string(schema)) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "schema_invalid",
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"schema": b64.StdEncoding.EncodeToString(schema),
	})
	return
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
