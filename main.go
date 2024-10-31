package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func create(c *gin.Context) {
	var userInput UserInput
	if err := c.BindJSON(&userInput); err != nil {
		return
	}
	client := &http.Client{}
	id, err := createDeposition(client, userInput)

	if err != nil {
		return
	}
	fmt.Println("created deposition", id)
}

func addFile(c *gin.Context) {
	var fileUpload FileUpload
	depositionID := c.Param("id")
	if err := c.BindJSON(&fileUpload); err != nil {
		return
	}
	client := &http.Client{}
	_, err := addFileToDeposition(client, depositionID, fileUpload)
	if err != nil {
		return
	}
}

// adds a deposition based on JSON received in the request body.
func createDepositionOneDep(c *gin.Context) {
	var userInput UserInput
	if err := c.BindJSON(&userInput); err != nil {
		return
	}
	client := &http.Client{}

	// Convert the user input to JSON
	_, err := createDeposition(client, userInput)

	if err != nil {
		return
	}
	// upload Files to deposition

}

func getDepositionByID(c *gin.Context) {
	id := c.Param("id")
	client := &http.Client{}
	jwtToken, err := os.ReadFile("bearer.jwt")
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}
	var bearer = "Bearer " + string(jwtToken)
	req, _ := http.NewRequest("GET", "https://onedep-depui-test.wwpdb.org/deposition/api/v1/depositions/"+id, nil)
	req.Header.Add("Authorization", bearer)
	fmt.Println("request:", req)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Errored when sending request to the server")
		fmt.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch deposition"})
		return
	}
	resp.Body.Close()
}

func main() {
	router := gin.Default()
	router.POST("/onedep", create)
	router.POST("/onedep/:id/files", addFile)
	router.GET("/onedep/:id", getDepositionByID)
	router.Run("localhost:8080")

	// file, err := os.Open("/Users/sofya/Downloads/cryosparc_P56_J33_010_volume_map_sharp.mrc")
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// defer file.Close()

}
