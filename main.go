package main

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"

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

// Convert multipart.File to *os.File by saving it to a temporary file
func convertMultipartFileToFile(file multipart.File) (*os.File, error) {
	tempFile, err := os.CreateTemp("", "uploaded-*")
	if err != nil {
		return nil, err
	}
	defer file.Close() // Close the multipart.File after copying

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
// @Param email formData string true "User's email"
// @Param experiments formData string true "Experiment type (e.g., single-particle analysis)"

// @Success 200 {string} string "Deposition ID"
// @Failure 400 {object} map[string]interface{} "Error response"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /onedep [post]
func Create(c *gin.Context) {

	var body onedep.RequestCreate

	// Bind JSON payload to the struct
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// email := body.Email
	email := "sofya.laskina@epfl.ch" //remove later
	experiments := []onedep.EmMethod{onedep.EmMethods[body.Method]}

	bearer := body.JWTToken
	depData := onedep.UserInfo{
		Email:       email,
		Users:       body.OrcidIds,
		Country:     "United States", // body.country
		Experiments: experiments,
	}

	client := &http.Client{}

	deposition, err := onedep.CreateDeposition(client, depData, bearer)
	if err != nil {
		errText := fmt.Errorf("failed to create deposition: %w", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": errText})
		return
	}
	c.JSON(http.StatusOK, gin.H{"depID": deposition.Id})
}

// AddFiles handles adding files to an existing deposition. It also opens a header of mrc files and extracts the pixel spacing
// @Summary Add file, pixel spacing, contour level and description to deposition in OneDep
// @Description Uploading file, and metadata to OneDep API.
// @Tags deposition
// @Accept multipart/form-data
// @Produce json
// @Param depositionID formData string true "Deposition ID to which a file should be uploaded"
// @Param file formData []file true "File to upload" collectionFormat(multi)
// @Param fileMetadata formData string true "File metadata as a JSON string"
// @Success 200 {string} string "File ID"
// @Failure 400 {object} map[string]interface{} "Error response"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /onedep/:depID/file [post]
func AddFile(c *gin.Context) {
	err := c.Request.ParseMultipartForm(10 << 20)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse Form data."})
		return
	}
	depID := c.Param("depID")
	bearer := c.PostForm("jwtToken")
	fileHeader := c.Request.MultipartForm.File["file"][0] //file

	metadataFilesStr := c.PostForm("fileMetadata") //files Metadata
	if metadataFilesStr == "{}" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing Files information."})
		return
	}

	var fileMetadata onedep.FileUpload
	err = json.Unmarshal([]byte(metadataFilesStr), &fileMetadata)
	if err != nil {
		errText := fmt.Errorf("error decoding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": errText})
	}

	file, err := fileHeader.Open()
	if err != nil {
		errText := fmt.Errorf("failed to open file header: %w", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": errText})
		return
	}
	defer file.Close()

	client := &http.Client{}
	var fileDeposited onedep.DepositionFile
	fileDeposited, err = onedep.AddFileToDeposition(client, depID, fileMetadata, file, bearer)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}
	for j := range onedep.NeedMeta {
		if fileMetadata.Type == onedep.NeedMeta[j] {
			onedep.AddMetadataToFile(client, fileDeposited, bearer)
		}
	}

	c.JSON(http.StatusOK, gin.H{"fileID": fileDeposited.Id})
}

// AddMetadata handles adding metadata to an existing deposition.
// @Summary Add a cif file with metadata to deposition in OneDep
// @Description Uploading metadata file to OneDep API. This is created by parsing the JSON metadata into the converter.
// @Tags deposition
// @Accept application/json
// @Produce json
// @Param depositionID formData string true "Deposition ID to which a file should be uploaded"
// @Success 200 {string} string "File ID"
// @Failure 400 {object} map[string]interface{} "Error response"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /onedep/:depID/metadata [post]
func AddMetadata(c *gin.Context) {
	depID := c.Param("depID")
	bearer := c.PostForm("jwtToken")

	var body map[string]interface{}

	// Bind JSON payload to the struct
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, ok := body["jwtToken"].(string)
	if !ok || token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid jwtToken"})
		return
	}
	// FIX ME add a OSCEM SCHEMA
	metadataStr, ok := body["metadata"].(string)
	if !ok || metadataStr == "{}" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid metadata"})
		return
	}

	// Parse  JSON string into the Metadata
	var scientificMetadata map[string]any
	err := json.Unmarshal([]byte(metadataStr), &scientificMetadata)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse scientific metadata to JSON."})
		return
	}
	// create a temporary cif file that will be used for a deposition
	fileScientificMeta, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create a file to write cif file with metadata."})
		return
	}
	fileScientificMetaPath := fileScientificMeta.Name()

	// convert OSCEM-JSON to mmCIF
	parser.EMDBconvert(
		scientificMetadata,
		"",
		"conversions.csv",
		"mmcif_pdbx_v50.dic",
		fileScientificMetaPath,
	)
	client := &http.Client{}
	metadataFile := onedep.FileUpload{
		Name: "metadata.cif",
		Type: "co-cif", // FIX ME add appropriate type once it's implemented in OneDep API
	}
	fileDeposited, err := onedep.AddCIFtoDeposition(client, depID, metadataFile, fileScientificMetaPath, bearer)
	if err != nil {
		errText := fmt.Errorf("failed to open temp file with annotated model and scientific metadata: %w", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": errText})
		return
	}

	//defer os.Remove(fileScientificMetaPath)
	fmt.Println(fileScientificMetaPath)
	c.JSON(http.StatusOK, gin.H{"fileID": fileDeposited.Id})
}

// AddCoordinates handles adding coordinates files to an existing deposition.
// @Summary Add coordinates and description to deposition in OneDep
// @Description Uploading file to OneDep API.
// @Tags deposition
// @Accept multipart/form-data
// @Produce json
// @Param depositionID formData string true "Deposition ID to which a file should be uploaded"
// @Param file formData []file true "File to upload" collectionFormat(multi)
// @Param fileMetadata formData string true "File metadata as a JSON string"
// @Success 200 {string} string "File ID"
// @Failure 400 {object} map[string]interface{} "Error response"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /onedep/:depID/pdb [post]
func AddCoordinates(c *gin.Context) {
	err := c.Request.ParseMultipartForm(10 << 20)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse Form data."})
		return
	}

	depID := c.Param("depID")
	bearer := c.PostForm("jwtToken")
	fileHeader := c.Request.MultipartForm.File["file"][0] //file

	metadataFilesStr := c.PostForm("fileMetadata") //files Metadata
	if metadataFilesStr == "{}" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing Files information."})
		return
	}

	var fileMetadata onedep.FileUpload
	err = json.Unmarshal([]byte(metadataFilesStr), &fileMetadata)
	if err != nil {
		errText := fmt.Errorf("error decoding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": errText})
	}

	// Extract entries from multipart form
	metadataStr := c.PostForm("metadata") //scientificMetadata
	if metadataStr == "{}" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing scientific metadata."})
		return
	}

	// Parse  JSON string into the Metadata
	var scientificMetadata map[string]any
	err = json.Unmarshal([]byte(metadataStr), &scientificMetadata)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse scientific metadata to JSON."})
		return
	}

	// create a temporary cif file that will be used for a deposition
	fileScientificMeta, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create a file to write cif file with metadata."})
		return
	}
	fileScientificMetaPath := fileScientificMeta.Name()

	file, err := fileHeader.Open()
	if err != nil {
		errText := fmt.Errorf("failed to open file header: %w", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": errText})
		return
	}
	defer file.Close()

	fileTmp, err := convertMultipartFileToFile(file)
	if err != nil {
		errText := fmt.Errorf("failed to open cif file to read coordinates: %w", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": errText})
		return
	}
	defer file.Close()

	fmt.Println(scientificMetadata)
	parser.PDBconvertFromFile(
		scientificMetadata,
		"",
		"conversions.csv",
		"mmcif_pdbx_v50.dic",
		fileTmp,
		fileScientificMetaPath,
	)

	client := &http.Client{}
	fileDeposited, err := onedep.AddCIFtoDeposition(client, depID, fileMetadata, fileScientificMetaPath, bearer)
	if err != nil {
		errText := fmt.Errorf("failed to open temp file with annotated model and scientific metadata: %w", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": errText})
		return
	}
	defer file.Close()

	// add details metadata

	if fileMetadata.Details != "" {
		onedep.AddMetadataToFile(client, fileDeposited, bearer)
	}
	c.JSON(http.StatusOK, gin.H{"fileID": fileDeposited.Id})
}

// FIX ME uncomment once pixel spacing and contour level are propagated correctly into the request, st files can be processed.
// _, err = onedep.ProcesseDeposition(client, deposition)
// if err != nil {
// 	c.JSON(http.StatusBadRequest, gin.H{"error": err})
// 	return
// }

//	returns the current version of the depositor
//
// @Summary Return current version
// @Description Create a new deposition by uploading experiments, files, and metadata to OneDep API.
// @Tags version
// @Produce json
// @Success 200 {string} string "Depositior version"
// @Failure 400 {object} map[string]interface{} "Error response"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /version [get]
func GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, version)

}
func main() {

	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:4200"},
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

	router.Run("localhost:8080")
}
