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
// @Description Create a new deposition by uploading experiments, files, and metadata to OneDep API.
// @Tags deposition
// @Accept multipart/form-data
// @Produce json
// @Param email formData string true "User's email"
// @Param experiments formData string true "Experiment type (e.g., single-particle analysis)"
// @Param file formData []file true "File(s) to upload" collectionFormat(multi)
// @Param metadata formData string true "Scientific metadata as a JSON string"
// @Param fileMetadata formData string true "File metadata as a JSON string"
// @Success 200 {string} string "Deposition ID"
// @Failure 400 {object} map[string]interface{} "Error response"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /onedep [post]
func Create(c *gin.Context) {
	err := c.Request.ParseMultipartForm(10 << 20)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse Form data."})
		return
	}
	// Extract entries from multipart form
	email := c.PostForm("email")
	experiment := onedep.EmMethods[c.PostForm("experiments")]
	experiments := []onedep.EmMethod{experiment}
	files := c.Request.MultipartForm.File["file"] //files
	metadataStr := c.PostForm("metadata")         //scientificMetadata
	if metadataStr == "{}" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing scientific metadata."})
		return
	}
	fmt.Println(email, experiments)
	metadataFilesStr := c.PostForm("fileMetadata") //files Metadata
	if metadataFilesStr == "{}" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing Files information."})
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

	var metadataFiles []onedep.FileUpload
	err = json.Unmarshal([]byte(metadataFilesStr), &metadataFiles)
	if err != nil {
		errText := fmt.Errorf("error decoding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": errText})
	}
	// FIXME: add calls to validation API before creating any deposition - beta

	// send a request to create a deposition:

	depData := onedep.UserInfo{
		Email:       "sofya.laskina@epfl.ch", //email,
		Users:       []string{"0009-0003-3665-5367"},
		Country:     "United States", // temporarily
		Experiments: experiments,
	}
	// fmt.Println(depData)
	client := &http.Client{}

	deposition, err := onedep.CreateDeposition(client, depData)
	if err != nil {
		errText := fmt.Errorf("failed to create deposition: %w", err)
		fmt.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": errText})
		return
	}
	// deposition := onedep.Deposition{
	// 	Id:    "D_800042",
	// 	Files: []onedep.DepositionFile{},
	// }
	// text, _ := fmt.Printf("deposition created in OneDep, id %v", deposition.Id)
	// c.JSON(http.StatusOK, gin.H{"messsage": "deposition created in OneDep"})
	// fmt.Println("created deposition", deposition.Id)
	var fileDeposited onedep.DepositionFile
	metadataTracked := false
	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			errText := fmt.Errorf("failed to open file header: %w", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": errText})
			return
		}
		defer file.Close()

		if metadataFiles[i].Type == "co-cif" {
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
			metadataTracked = true
			fileDeposited, err = onedep.AddCIFtoDeposition(client, deposition, metadataFiles[i], fileScientificMetaPath)
			if err != nil {
				errText := fmt.Errorf("failed to open temp file with annotated model and scientific metadata: %w", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": errText})
				return
			}
			defer file.Close()
		} else {
			fileDeposited, err = onedep.AddFileToDeposition(client, deposition, metadataFiles[i], file)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err})
				return
			}
		}
		deposition.Files = append(deposition.Files, fileDeposited)

		for j := range onedep.NeedMeta {
			if metadataFiles[i].Type == onedep.NeedMeta[j] {
				onedep.AddMetadataToFile(client, fileDeposited)
			}
		}
	}
	// if no cif was provided, then extra metadata file needs to be created
	if !metadataTracked {
		parser.EMDBconvert(
			scientificMetadata,
			"",
			"conversions.csv",
			"mmcif_pdbx_v50.dic",
			fileScientificMetaPath,
		)

		metadataFile := onedep.FileUpload{
			Name: "metadata.cif",
			Type: "co-cif", // FIX ME add aproprioate type once it's implemeted in OneDep API
		}
		fileDeposited, err = onedep.AddCIFtoDeposition(client, deposition, metadataFile, fileScientificMetaPath)
		if err != nil {
			errText := fmt.Errorf("failed to open temp file with annotated model and scientific metadata: %w", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": errText})
			return
		}
		deposition.Files = append(deposition.Files, fileDeposited)
	}
	//defer os.Remove(fileScientificMetaPath)
	fmt.Println(fileScientificMetaPath)
	c.JSON(http.StatusOK, deposition.Id)

	// FIX ME uncomment once pixel spacing and contour level are propagated correctly into the request, st files can be processed.
	// _, err = onedep.ProcesseDeposition(client, deposition)
	// if err != nil {
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": err})
	// 	return
	// }

}

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

	router.Run("localhost:8080")
}
