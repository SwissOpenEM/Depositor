package onedep

import (
	"bytes"
	"mime/multipart"
)

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

type RequestCreate struct {
	Email       string   `json:"email" binding:"required,email"`
	OrcidIds    []string `json:"orcidIds" binding:"required"`
	Password    string   `json:"password"`
	Country     string   `json:"country" binding:"required"`
	Method      string   `json:"method" binding:"required"`
	JWTToken    string   `json:"jwtToken" binding:"required"`
	Coordinates bool     `json:"coordinates"`
}

type FileMetadata struct {
	FileName string  `json:"name"`
	FileType string  `json:"type"`
	Contour  float32 `json:"contour,omitempty"`
	Details  string  `json:"details"`
}
type ResponseType struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
type CreatedDeposition struct {
	DepID string `json:"depID"`
}
type UploadedFile struct {
	DepID  string `json:"depID"`
	FileID string `json:"FileID"`
}
type EmMethod struct {
	Type        string `json:"type"`
	Subtype     string `json:"subtype,omitempty"`
	Coordinates bool   `json:"coordinates"`
	SfOnly      bool   `json:"sf_only"`
	RelatedEmdb string `json:"related_emdb,omitempty"`
}
type EmMethodExtended struct {
	Type        string `json:"type"`
	Subtype     string `json:"subtype,omitempty"`
	EMDBrelated string `json:"related_emdb,omitempty"`
	BMRBrelated string `json:"related_bmrb,omitempty"`
}

var EmMethods = map[string]EmMethod{
	"helical":                  {Type: "em", Subtype: "helical"},
	"single-particle":          {Type: "em", Subtype: "single"},
	"subtomogram-averaging":    {Type: "em", Subtype: "subtomogram"},
	"tomogram":                 {Type: "em", Subtype: "tomography"},
	"electron-crystallography": {Type: "ec"},
}

type UserInfo struct {
	Email       string     `json:"email"`
	Users       []string   `json:"users"`
	Country     string     `json:"country"`
	Experiments []EmMethod `json:"experiments"`
	Password    string     `json:"password,omitempty"`
}

type FileUpload struct {
	Name    string     `json:"name"`
	Type    OneDepType `json:"type"`
	Contour float32    `json:"contour"`
	Details string     `json:"details"`
}

type DepositionFile struct {
	DId          string
	Id           int
	Name         string
	Type         string
	PixelSpacing [3]float32
	ContourLevel float32
	Details      string
}
type FileDepositionRequest struct {
	Body   *bytes.Buffer
	Writer *multipart.Writer
}
type Deposition struct {
	Id    string
	Files []DepositionFile
	// MetadataFile string
}

type OneDepError struct {
	Code    string           `json:"code"`
	Message string           `json:"message"`
	Extras  map[string][]any `json:"extras,omitempty"`
}
type Status string

const (
	AUCO Status = "auco"
	AUTH Status = "auth"
	AUXS Status = "auxs"
	AUXU Status = "auxu"
	DEP  Status = "dep"
	HOLD Status = "hold"
	HPUB Status = "hpub"
	OBS  Status = "obs"
	POLC Status = "polc"
	PROC Status = "proc"
	REL  Status = "rel"
	REPL Status = "repl"
	REUP Status = "reup"
	WAIT Status = "wait"
	WDRN Status = "wdrn"
)

type OneDepType string

const (
	ADD_MAP     OneDepType = "add-map"
	CO_CIF      OneDepType = "co-cif"
	MD_CIF      OneDepType = "md-cif"
	CO_PDB      OneDepType = "co-pdb"
	FSC_XML     OneDepType = "fsc-xml"
	HALF_MAP    OneDepType = "half-map"
	IMG_EMDB    OneDepType = "img-emdb"
	LAYER_LINES OneDepType = "layer-lines"
	MASK_MAP    OneDepType = "mask-map"
	NM_AUX_AMB  OneDepType = "nm-aux-amb"
	NM_AUX_GRO  OneDepType = "nm-aux-gro"
	NM_PEA_ANY  OneDepType = "nm-pea-any"
	NM_RES_AMB  OneDepType = "nm-res-amb"
	NM_RES_BIO  OneDepType = "nm-res-bio"
	NM_RES_CHA  OneDepType = "nm-res-cha"
	NM_RES_CNS  OneDepType = "nm-res-cns"
	NM_RES_CYA  OneDepType = "nm-res-cya"
	NM_RES_DYN  OneDepType = "nm-res-dyn"
	NM_RES_GRO  OneDepType = "nm-res-gro"
	NM_RES_ISD  OneDepType = "nm-res-isd"
	NM_RES_OTH  OneDepType = "nm-res-oth"
	NM_RES_ROS  OneDepType = "nm-res-ros"
	NM_RES_SYB  OneDepType = "nm-res-syb"
	NM_RES_XPL  OneDepType = "nm-res-xpl"
	NM_SHI      OneDepType = "nm-shi"
	NM_UNI_NEF  OneDepType = "nm-uni-nef"
	NM_UNI_STR  OneDepType = "nm-uni-str"
	VO_MAP      OneDepType = "vo-map"
	XA_MAT      OneDepType = "xa-mat"
	XA_PAR      OneDepType = "xa-par"
	XA_TOP      OneDepType = "xa-top"
	XS_CIF      OneDepType = "xs-cif"
	XS_MTZ      OneDepType = "xs-mtz"
)

// here is what I get without using response definition from apiary. It differs from the one online and especially the experiments type is much different, there is an option for coordinates.
// map[created:2025-01-28T09:32:27
// email:sofya.laskina@epfl.ch
// entry_id:?
// experiments:[map[coordinates:true related_emdb:<nil> sf_only:false subtype:helical type:em]]
// id:D_800143
// last_login:2025-01-28T09:32:27.376473
// site:RCSB
// status:DEP
// title:?]
type DepositionResponse struct {
	Email       string      `json:"email"`
	Id          string      `json:"id"`
	PDBid       string      `json:"pdb_id"`
	EMDBid      string      `json:"emdb_id"`
	BMRBid      string      `json:"bmrb_id"`
	Title       string      `json:"title"`
	HoldExpDate string      `json:"hold_exp_date"`
	Created     string      `json:"created"`
	LastLogin   string      `json:"last_login"`
	Site        string      `json:"site"`
	SiteUrl     string      `json:"site_url"`
	Status      Status      `json:"status"`
	Experiments any         `json:"experiments"` // FIX ME actual response type does not correspond to the documented.
	Errors      OneDepError `json:"errors"`
	Code        string      `json:"code"`
	Message     string      `json:"message"`
}

type SpacingType struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	Z float32 `json:"z"`
}
type VoxelType struct {
	Spacing SpacingType `json:"spacing"`
	Contour float32     `json:"contour"`
}

type EmMapMetadata struct {
	Voxel       VoxelType `json:"voxel"`
	Description string    `json:"description"`
}

// map[
// created:Tuesday, January 28, 2025 10:01:02
// errors:[map[code:upload_error message:Please provide/select one coordinate file.]
//
//	map[code:upload_error message:Please provide/select 1 primary map]
//	map[code:upload_error message:Please provide your half maps.]
//	map[code:upload_error message:Please provide/select one image of your map.]]
//
// id:649 name:mainMap.mrcPi8Jno8B
// type:vo-map
// warnings:[map[code:upload_warning message:Deposition of a FSC file is strongly encouraged.]]]

type FileResponse struct {
	Id       int            `json:"id"`
	Name     string         `json:"name"` // not there
	Type     OneDepType     `json:"type"`
	Created  string         `json:"created"`
	Metadata map[string]any `json:"metadata"` // not there
	Errors   any            `json:"errors"`   // FIX ME actual response type does not correspond to the documented.
	Warnings any            `json:"warnings"` // FIX ME actual response type does not correspond to the documented.
}

var NeedMeta = []string{"vo-map", "add-map", "half-map", "mask-map"}
