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
)

var version string = "DEV"

var allowedType = []string{"vo-map", "add-map", "co-cif", "co-pdb", "fsc-xml", "half-map", "img-emdb", "mask-map"}
var needMeta = []string{"vo-map", "add-map", "half-map", "mask-map"}

func convertToOSFile(fileHeader *multipart.FileHeader) (*os.File, error) {
	// Open the uploaded file
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file header: %w", err)
	}
	defer file.Close()

	// Create a temporary file in the system's temp directory
	tempFile, err := os.CreateTemp("", fileHeader.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Copy the contents of the uploaded file to the temporary file
	if _, err := io.Copy(tempFile, file); err != nil {
		tempFile.Close()
		return nil, fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Rewind the file pointer to the start
	if _, err := tempFile.Seek(0, 0); err != nil {
		tempFile.Close()
		return nil, fmt.Errorf("failed to seek to start of file: %w", err)
	}

	return tempFile, nil
}

func create(c *gin.Context) {
	err := c.Request.ParseMultipartForm(10 << 20) // 10 MB max memory
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse Form data."})
		return
	}
	// Extract entries from multipart form
	email := c.PostForm("email")
	experiment, _ := onedep.EmMethods[c.PostForm("experiments")]
	experiments := []onedep.EmMethod{experiment}

	metadataStr := c.PostForm("metadata")
	if metadataStr == "{}" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing scientific metadata."})
		return
	}
	metadataFilesStr := c.PostForm("fileMetadata")
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
	var metadataFiles any
	err = json.Unmarshal([]byte(metadataFilesStr), &metadataFiles)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse Files information to JSON."})
		return
	}

	files := c.Request.MultipartForm.File["file"]
	// var osFiles []*os.File
	for _, fileHeader := range files {
		osFile, err := convertToOSFile(fileHeader)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error converting file"})
			fmt.Println("Error converting file:", err)
			return
		}
		// osFiles = append(osFiles, osFile)

		a, err := onedep.GetMeta(osFile)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(a, email, experiments, metadata)
		defer osFile.Close()
	}

	// var userInput onedep.ScicatEM
	// err := json.NewDecoder(c.Request.Body).Decode(&userInput)
	// if err != nil {
	// 	fmt.Println("can't decode input")
	// 	return
	// }

	// if err := c.BindJSON(&userInput); err != nil {
	// 	fmt.Println("can't bind input")
	// 	return
	// }
	// fmt.Println(userInput)

	// send a request to create a deposition:
	depData := onedep.UserInfo{
		Email:       "some@email.ch", //email,
		Users:       []string{"0000-0000-0000-00XX"},
		Country:     "United States", // temporarily
		Experiments: experiments,
	}
	client := &http.Client{}
	deposition, err := onedep.CreateDeposition(client, depData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}
	fmt.Println("created deposition", deposition.Id)
	// // FIXME: invoke the converter to mmCIF to produce metadata file
	// // FIXME: add calls to validation API
	// for f := range userInput.Files {
	// 	// for all files provideed, upload them to onedep an dadd to struct
	// 	fileDeposited, err := onedep.AddFileToDeposition(client, deposition, userInput.Files[f])
	// 	if err != nil {
	// 		return
	// 	}
	// 	deposition.Files = append(deposition.Files, fileDeposited)
	// 	for i := range needMeta {
	// 		if userInput.Files[f].Type == needMeta[i] {
	// 			onedep.AddMetadataToFile(client, fileDeposited)
	// 		}
	// 	}
	// 	// FIXME:  need to add checks if all necessary files are preaent based on deposition type
	// }

	// _, err = onedep.ProcesseDeposition(client, deposition)
	// if err != nil {
	// 	return
	// }

	// os.Remove(tempFile.Name())
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
