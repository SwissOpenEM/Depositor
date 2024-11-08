package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"tasks/depositions/onedep"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var version string = "DEV"

func create(c *gin.Context) {
	err := c.Request.ParseMultipartForm(10 << 20) // 10 MB max memory
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
	var metadata any
	err = json.Unmarshal([]byte(metadataStr), &metadata)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse scientific metadata to JSON."})
		return
	}

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
	client := &http.Client{}

	deposition, err := onedep.CreateDeposition(client, depData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}
	// fmt.Println("created deposition", deposition.Id)

	// // FIXME: invoke the converter to mmCIF to produce metadata file

	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			errText := fmt.Errorf("failed to open file header: %w", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": errText})
			return
		}
		defer file.Close()

		fileDeposited, err := onedep.AddFileToDeposition(client, deposition, metadataFiles[i], file)
		// osFile, err := handleOSFile(fileHeader)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err})
			return
		}
		deposition.Files = append(deposition.Files, fileDeposited)

		for j := range onedep.NeedMeta {
			if metadataFiles[i].Type == onedep.NeedMeta[j] {
				onedep.AddMetadataToFile(client, fileDeposited)
			}
		}
	}

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
