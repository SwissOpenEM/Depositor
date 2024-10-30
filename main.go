package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type Experiment struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
}

type UserInput struct {
	Email       string       `json:"email"`
	Users       []string     `json:"users"`
	Country     string       `json:"country"`
	Experiments []Experiment `json:"experiments"`
}
type FileUpload struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	File    string `json:"file"`
	Contour string `json:"contour"`
}

func decodeDid(resp *http.Response) (string, error) {
	type DidType struct {
		Did string `json:"id"`
	}
	var d DidType
	decoder := json.NewDecoder(resp.Body)
	err := decoder.Decode(&d)

	if err != nil {
		return "", fmt.Errorf("could not decode id from deposition entry: %v", err)
	}
	fmt.Println("Decoded struct:", d)
	return d.Did, nil
}

func decodeFid(resp *http.Response) (string, error) {
	type FidType struct {
		Fid int32 `json:"id"`
	}
	var f FidType
	decoder := json.NewDecoder(resp.Body)
	_ = decoder.Decode(&f)
	var result map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	// if err != nil {
	// 	 return "", fmt.Errorf("could not decode id from deposition entry: %v", err)
	// 	// errors point to the entries still missing for the whole deposition and do not represent the error for this exact request
	// }

	return fmt.Sprintf("%d", f.Fid), nil
}

func createDeposition(client *http.Client, userInput UserInput) (string, error) {
	depositionId := ""
	// Convert the user input to JSON
	jsonInput, err := json.Marshal(userInput)
	if err != nil {
		return "", err
	}

	req, _ := http.NewRequest("POST", "https://onedep-depui-test.wwpdb.org/deposition/api/v1/depositions/new", bytes.NewBuffer(jsonInput))

	jwtToken, err := os.ReadFile("bearer.jwt")
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}
	var bearer = "Bearer " + string(jwtToken)
	req.Header.Add("Authorization", bearer)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Errored when sending request to the server")
		fmt.Println(err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		depositionId, err = decodeDid(resp)
		if err != nil {
			return "", err
		}
	}
	//else...

	return depositionId, nil
}
func addFileToDeposition(client *http.Client, depositionId string, fileUpload FileUpload) (string, error) {
	fileId := ""
	// Create a buffer to hold the multipart form data
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	// Add the required fields
	if err := writer.WriteField("name", fileUpload.Name); err != nil {
		return "", err
	}
	if err := writer.WriteField("type", fileUpload.Type); err != nil {
		return "", err
	}

	// Add the actual file
	file, err := os.Open(fileUpload.File)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// extract pixel spacing necessary to upload metadata
	pixelSpacing := getMeta(file)

	part, err := writer.CreateFormFile("file", filepath.Base(fileUpload.File))
	if err != nil {
		return "", err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return "", err
	}

	// Close the writer to finalize the multipart form
	err = writer.Close()
	if err != nil {
		return "", err
	}

	// Prepare the request
	url := "https://onedep-depui-test.wwpdb.org/deposition/api/v1/depositions/" + depositionId + "/files/"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", err
	}

	jwtToken, err := os.ReadFile("bearer.jwt")
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}
	var bearer = "Bearer " + string(jwtToken)

	req.Header.Add("Authorization", bearer)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request to the server")
		fmt.Println(err)
		return "", err
	}
	defer resp.Body.Close()
	// bodyBytes, _ := io.ReadAll(resp.Body)
	// fmt.Println("Response body:", string(bodyBytes))
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		fileId, err = decodeFid(resp)
		if err != nil {
			return "", err
		}
		fmt.Println("fileID:", fileId)
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Println(resp.StatusCode)
		return "", fmt.Errorf("server responded with status %s: %s", resp.Status, string(bodyBytes))
	}

	// Prepare metadata request
	data := map[string]interface{}{
		"voxel": map[string]interface{}{
			"spacing": map[string]float32{
				"x": pixelSpacing[0],
				"y": pixelSpacing[1],
				"z": pixelSpacing[2],
			},
			"contour": fileUpload.Contour, // set your contour value here
		},
		"description": "",
	}

	jsonBody, err := json.Marshal(data)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
	}
	urlFileMeta := "https://onedep-depui-test.wwpdb.org/deposition/api/v1/depositions/" + depositionId + "/files/" + fileId + "/metadata"
	req2, err := http.NewRequest("POST", urlFileMeta, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	req2.Header.Add("Authorization", bearer)
	req2.Header.Set("Content-Type", "application/json")

	fmt.Println("send the request to upload metedata", req2)
	// Send the request
	resp2, err := client.Do(req2)
	if err != nil {
		fmt.Println("Error sending request to the server")
		fmt.Println(err)
		return "", err
	}
	defer resp2.Body.Close()

	bodyBytes, _ := io.ReadAll(resp2.Body)
	fmt.Println("Response2 body:", string(bodyBytes))

	if resp2.StatusCode == 200 || resp2.StatusCode == 201 {
		return fileId, nil
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("server responded with status %s: %s", resp.Status, string(bodyBytes))
	}
}

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

// these constants are described in the definition of CCP4 format used for mrc files
const (
	headerSize    = 1024
	wordSize      = 4
	modeWord      = 3
	samplingWord  = 7
	cellDimWord   = 10
	numberOfWords = 56
)

type TypeCaster func(data []byte) interface{}

var typeMap = map[uint32]TypeCaster{
	0: func(data []byte) interface{} {
		return int8(data[0]) // 8-bit signed integer
	},
	1: func(data []byte) interface{} {
		return int16(binary.LittleEndian.Uint16(data)) // 16-bit signed integer
	},
	2: func(data []byte) interface{} {
		return math.Float32frombits(binary.LittleEndian.Uint32(data)) // 32-bit signed real
	},
	3: func(data []byte) interface{} {
		// Complex 16-bit integers (for simplicity, return raw data)
		return []int16{int16(binary.LittleEndian.Uint16(data[:2])), int16(binary.LittleEndian.Uint16(data[2:]))}
	},
	4: func(data []byte) interface{} {
		// Complex 32-bit reals
		return []float32{math.Float32frombits(binary.LittleEndian.Uint32(data[:4])), math.Float32frombits(binary.LittleEndian.Uint32(data[4:]))}
	},
	6: func(data []byte) interface{} {
		return binary.LittleEndian.Uint16(data) // 16-bit unsigned integer
	},
	12: func(data []byte) interface{} {
		return math.Float32frombits(binary.LittleEndian.Uint32(data)) // 16-bit float (IEEE754)
	},
	101: func(data []byte) interface{} {
		// 4-bit data packed two per byte (handle accordingly, for now just returning raw bytes)
		return data
	},
}

func getMeta(file *os.File) []float32 {
	// https://bio3d.colorado.edu/imod/betaDoc/mrc_format.txt
	// words I care about: Mode(4),	sampling along axes of unit cell (8-10), cell dimensions in angstroms(11-13) --> pixel spacing = cell dim/sampling
	header := make([]byte, headerSize)
	_, err := file.Read(header)
	if err != nil {
		log.Fatalf("Failed to read header: %v", err)
	}

	var mode uint32 = binary.LittleEndian.Uint32(header[modeWord*wordSize : modeWord*wordSize+wordSize])
	var pixelSpacing [3]float32
	var cellDim [3]float32
	if castFunc, ok := typeMap[mode]; ok {
		for i := 0; i < 3; i++ {
			cellDim[i] = castFunc(header[(cellDimWord+i)*wordSize : (cellDimWord+i)*wordSize+wordSize]).(float32)
			sampling := binary.LittleEndian.Uint32(header[(samplingWord+i)*wordSize : (samplingWord+i)*wordSize+wordSize])
			// Calculate pixel spacing
			pixelSpacing[i] = cellDim[i] / float32(sampling)
		}

	} else {
		fmt.Println("Mode not described in EM community")
	}

	return pixelSpacing[:]
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
