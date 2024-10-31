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

type DepositionFile struct {
	DId          string
	Id           string
	Type         string
	PixelSpacing [3]float32
	ContourLevel float32
	Description  string
}

type Deposition struct {
	Id           string
	Files        []DepositionFile
	MetadataFile string
}
