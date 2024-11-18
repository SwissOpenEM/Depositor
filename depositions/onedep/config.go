package onedep

const baseURL = "https://onedep-depui-test.wwpdb.org/deposition/api/v1/depositions/"

// these constants are described in the definition of CCP4 format used for mrc files
const (
	headerSize    = 1024
	wordSize      = 4
	modeWord      = 3
	samplingWord  = 7
	cellDimWord   = 10
	numberOfWords = 56
)

type EmMethod struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"`
}

var EmMethods = map[string]EmMethod{
	"helical":                  {Type: "em", Subtype: "helical"},
	"single-particle":          {Type: "em", Subtype: "single"},
	"subtomogram-averaging":    {Type: "em", Subtype: "subtomogram"},
	"tomogram":                 {Type: "em", Subtype: "tomography"},
	"electron-cristallography": {Type: "ec"},
}

//	type ScicatEM struct {
//		Email      string
//		Metadata   string
//		Experiment string
//		Files      []*multipart.FileHeader
//	}
type UserInfo struct {
	Email       string     `json:"email"`
	Users       []string   `json:"users"`
	Country     string     `json:"country"`
	Experiments []EmMethod `json:"experiments"`
}

//	type UserInput struct {
//		Email       string       `json:"email"`
//		Users       []string     `json:"users"`
//		Country     string       `json:"country"`
//		Experiments []Experiment `json:"experiments"`
//		Files       []FileUpload `json:"files"`
//	}
type FileUpload struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// File    string  `json:"file"`
	Contour float32 `json:"contour"`
	Details string  `json:"details"`
}

type DepositionFile struct {
	DId          string
	Id           string
	Name         string
	Type         string
	PixelSpacing [3]float32
	ContourLevel float32
	Details      string
}

type Deposition struct {
	Id    string
	Files []DepositionFile
	// MetadataFile string
}

var NeedMeta = []string{"vo-map", "add-map", "half-map", "mask-map"}
