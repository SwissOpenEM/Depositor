package main

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
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
var PORT string = getEnv("PORT", "8888")
var ALLOW_ORIGINS string = getEnv("ALLOW_ORIGINS", "http://localhost:4200")

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

// Create handles the creation of a new deposition
// @Summary Create a new deposition to OneDep
// @Description Create a new deposition by uploading experiment and user details to OneDep API.
// @Tags deposition
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
// @Tags deposition
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
	fileHeader := c.Request.MultipartForm.File["file"][0] //file
	metadataFilesStr := c.PostForm("fileMetadata")        //files Metadata

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
	fileName := strings.Split(fileHeader.Filename, ".")
	extension := fileName[len(fileName)-1]

	file, err := fileHeader.Open()
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
// @Tags deposition
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
	fileScientificMeta, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "cif_creation_fails",
			"message": "Failed to create a file to write cif file with metadata.",
		})
		return
	}
	fileScientificMetaPath := fileScientificMeta.Name()

	// convert OSCEM-JSON to mmCIF
	cifText, err := parser.EMDBconvert(
		scientificMetadata,
		"",
		"conversions.csv",
		"mmcif_pdbx_v50.dic",
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "conversion_to_cif_fails",
			"message": err.Error(),
		})
		return
	}
	err = parser.WriteCif(cifText, fileScientificMetaPath)
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
	cifFile, err := os.Open(fileScientificMetaPath)
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
	defer os.Remove(fileScientificMetaPath)

	c.JSON(http.StatusOK, uploadedFile)
}

// AddCoordinates handles adding coordinates files to an existing deposition.
// @Summary Add coordinates and description to deposition in OneDep
// @Description Uploading file to OneDep API.
// @Tags deposition
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
	fileHeader := c.Request.MultipartForm.File["file"][0] //file

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
	fileScientificMeta, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "cif_invalid",
			"message": "Failed to create a file to write new cif file.",
		})
		return
	}
	fileScientificMetaPath := fileScientificMeta.Name()

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "file_invalid",
			"message": fmt.Sprintf("Failed to open file header: %v", err.Error()),
		})
		return
	}
	defer file.Close()

	fileTmp, err := convertMultipartFileToFile(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "file_invalid",
			"message": fmt.Sprintf("failed to open cif file to read coordinates: %v", err.Error()),
		})
		return
	}
	defer file.Close()

	cifText, err := parser.PDBconvertFromFile(
		scientificMetadata,
		"",
		"conversions.csv",
		"mmcif_pdbx_v50.dic",
		fileTmp,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "conversion_to_cif_fails",
			"message": err.Error(),
		})
		return
	}

	err = parser.WriteCif(cifText, fileScientificMetaPath)
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
	cifFile, err := os.Open(fileScientificMetaPath)
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
	defer os.Remove(fileScientificMetaPath)

	c.JSON(http.StatusOK, uploadedFile)

}

// DownloadMetadata handles getting a cif file with metadata.
// @Summary Get a cif file with metadata for manual deposition in OneDep
// @Description Downloading a metadata file. Invokes converter and starts download.
// @Tags deposition
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
	fileScientificMeta, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "cif_creation_fails",
			"message": "Failed to create a file to write cif file with metadata.",
		})
		return
	}
	fileScientificMetaPath := fileScientificMeta.Name()

	// convert OSCEM-JSON to mmCIF
	cifText, err := parser.EMDBconvert(
		scientificMetadata,
		"",
		"conversions.csv",
		"mmcif_pdbx_v50.dic",
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "conversion_to_cif_fails",
			"message": err.Error(),
		})
		return
	}
	err = parser.WriteCif(cifText, fileScientificMetaPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "writing_cif_fails",
			"message": err.Error(),
		})
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.FileAttachment(fileScientificMetaPath, "metadata.cif")

	defer os.Remove(fileScientificMetaPath)

}

// DownloadCoordinates handles parsing an existing cif and metadata and downloading a new cif file.
// @Summary Get a cif file with metadata and coordinates for manual deposition in OneDep
// @Description Downloading a metadata file. Invokes converter and starts download.
// @Tags deposition
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

	fileHeader := c.Request.MultipartForm.File["file"][0]

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
	fileScientificMeta, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "cif_invalid",
			"message": "Failed to create a file to write new cif file.",
		})
		return
	}
	fileScientificMetaPath := fileScientificMeta.Name()

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "file_invalid",
			"message": fmt.Sprintf("Failed to open file header: %v", err.Error()),
		})
		return
	}
	defer file.Close()

	fileTmp, err := convertMultipartFileToFile(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "file_invalid",
			"message": fmt.Sprintf("failed to open cif file to read coordinates: %v", err.Error()),
		})
		return
	}
	defer file.Close()

	cifText, err := parser.PDBconvertFromFile(
		scientificMetadata,
		"",
		"conversions.csv",
		"mmcif_pdbx_v50.dic",
		fileTmp,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "conversion_to_cif_fails",
			"message": err.Error(),
		})
		return
	}

	err = parser.WriteCif(cifText, fileScientificMetaPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "writing_cif_fails",
			"message": err.Error(),
		})
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.FileAttachment(fileScientificMetaPath, "metadata.cif")

	defer os.Remove(fileScientificMetaPath)
}

// Create handles the creation of a new deposition
// @Summary Process deposition to OneDep
// @Description Process a deposition in OneDep API.
// @Tags deposition
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

//	returns the current version of the depositor
//
// @Summary Return current version
// @Description Create a new deposition by uploading experiments, files, and metadata to OneDep API.
// @Tags version
// @Produce json
// @Success 200 {string} string "Depositior version"
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

	if err := router.Run(":" + PORT); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		os.Exit(1)
	}
}
