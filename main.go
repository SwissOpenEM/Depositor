package main

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"tasks/depositions/onedep"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	parser "github.com/osc-em/converter-OSCEM-to-mmCIF/parser"
)

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

func create(c *gin.Context) {
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
	fmt.Println(scientificMetadata)
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
	fmt.Println(depData)
	client := &http.Client{}

	// deposition, err := onedep.CreateDeposition(client, depData)
	// if err != nil {
	// 	errText := fmt.Errorf("failed to create deposition: %w", err)
	// 	fmt.Println(err)
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": errText})
	// 	return
	// }
	deposition := onedep.Deposition{
		Id:    "D_800041",
		Files: []onedep.DepositionFile{},
	}

	// fmt.Println("created deposition", deposition.Id)

	metadataTracked := false
	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			errText := fmt.Errorf("failed to open file header: %w", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": errText})
			return
		}
		defer file.Close()
		var fileDeposited onedep.DepositionFile
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
			// fileDeposited, err = onedep.AddFileToDeposition(client, deposition, metadataFiles[i], file)
			// if err != nil {
			// 	c.JSON(http.StatusBadRequest, gin.H{"error": err})
			// 	return
			// }
			fmt.Println("skipping..")
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
	}
	//FIX ME deposition of metadata file to cif after fix from EBI  (and then remove)
	//defer os.Remove(fileScientificMetaPath)
	fmt.Println(fileScientificMetaPath)

	_, err = onedep.ProcesseDeposition(client, deposition)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}

}

func getVersion(c *gin.Context) {
	c.JSON(http.StatusOK, version)

}
func main() {
	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:4200"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
	}))
	router.GET("/version", getVersion)
	router.POST("/onedep", create)
	router.GET("/onedep", create)
	router.Run("localhost:8080")
}

// AllowOrigins:     []string{"https://foo.com"},
// AllowMethods:     []string{"PUT", "PATCH"},
// AllowHeaders:     []string{"Origin"},
// ExposeHeaders:    []string{"Content-Length"},
// AllowCredentials: true,
// AllowOriginFunc: func(origin string) bool {
// 	return origin == "https://github.com"
// },
// MaxAge: 12 * time.Hour,
