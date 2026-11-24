package onedep

import (
	"bytes"
	"mime/multipart"
)

type UserInput struct {
	Method      string   `json:"method"`
	Coordinates *bool    `json:"coordinates,omitempty"`
	Country     string   `json:"country"`
	Email       string   `json:"email"`
	Password    *string  `json:"password"`
	OrcidIds    []string `json:"orcidIds"`
	JwtToken    string   `json:"jwtToken"`
}

type RequestError struct {
	Status  string
	Message string
}

func (e *RequestError) Error() string {
	return e.Message
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

type FileUpload struct {
	Name    string     `json:"name"`
	Type    OneDepType `json:"type"`
	Contour float32    `json:"contour"`
	Details string     `json:"details"`
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
