package empiar

type CrossReference struct {
	Name string `json:"name"`
}
type BiostudiesReference struct {
	Name string `json:"name"`
}
type IdrReference struct {
	Name string `json:"name"`
}
type EmpiarReference struct {
	Name string `json:"name"`
}
type Workflow struct {
	Url  string `json:"url"`
	Type int    `json:"type"`
}

type Author struct {
	Name        string `json:"name"`
	OrderId     int    `json:"order_id"`
	AuthorOrcid string `json:"author_orcid"`
}

type ExtendedAuthor struct {
	AuthorOrcid  string      `json:"author_orcid"`
	FirstName    string      `json:"first_name"`
	MiddleName   string      `json:"middle_name"`
	LastName     string      `json:"last_name"`
	Orgainzation string      `json:"organization"`
	Street       string      `json:"street"`
	City         string      `json:"city"`
	State        string      `json:"state"`
	PostalCode   string      `json:"postal_code"`
	Telephone    string      `json:"telephone"`
	Fax          string      `json:"fax"`
	Email        string      `json:"email"`
	Country      CountryCode `json:"country"`
}

type WorkflowFile struct {
	Path string `json:"path"`
}

type Imageset struct {
	Name                       string  `json:"name"`
	Directory                  string  `json:"directory,omitempty"`
	Category                   string  `json:"category,omitempty"`
	HeaderFormat               string  `json:"header_format,omitempty"`
	DataFormat                 string  `json:"data_format,omitempty"`
	NumImagesOrTiltSeries      int     `json:"num_images_or_tilt_series,omitempty"`
	FramesPerImage             int     `json:"frames_per_image,omitempty"`
	VoxelType                  string  `json:"voxel_type,omitempty"`
	PixelWidth                 float64 `json:"pixel_width,omitempty"`
	PixelHeight                float64 `json:"pixel_height,omitempty"`
	Details                    string  `json:"details,omitempty"`
	ImageWidth                 int     `json:"image_width,omitempty"`
	ImageHeight                int     `json:"image_height,omitempty"`
	MicrographsFilePattern     string  `json:"micrographs_file_pattern,omitempty"`
	PickedParticlesFilePattern string  `json:"picked_particles_file_pattern,omitempty"`
	PickedParticlesDirectory   string  `json:"picked_particles_directory,omitempty"`
}
type CitationAuthor struct {
	Name string `json:"name"`
}
type Citation struct {
	Authors             []Author    `json:"authors"`
	Editors             []Author    `json:"editors"`
	Published           bool        `json:"published"`
	Preprint            bool        `json:"preprint"`
	JournalCitation     bool        `json:"j_or_nj_citation"`
	Title               string      `json:"title"`
	Volume              string      `json:"volume"`
	Country             CountryCode `json:"country"`
	FirstPage           string      `json:"first_page"`
	LastPage            string      `json:"last_page"`
	Year                string      `json:"year"`
	Language            string      `json:"language"`
	Doi                 string      `json:"doi"`
	Pubmedid            string      `json:"pubmedid"`
	Details             string      `json:"details"`
	BookChapterTitle    string      `json:"book_chapter_title"`
	Publisher           string      `json:"publisher"`
	PublicationLocation string      `json:"publication_location"`
	Journal             string      `json:"journal"`
	JournalAbbreviation string      `json:"journal_abbreviation"`
	Issue               string      `json:"issue"`
}

type EMPIARdeposition struct {
	Title                  string                `json:"title"`
	ReleaseDate            string                `json:"release_date"`
	ExperimentType         int                   `json:"experiment_type"`
	Scale                  string                `json:"scale"`
	CrossReferences        []CrossReference      `json:"cross_references"`
	BiostudiesReferences   []BiostudiesReference `json:"biostudies_references"`
	IdrReferences          []IdrReference        `json:"idr_references"`
	EmpiarReferences       []EmpiarReference     `json:"empiar_references"`
	Workflows              []Workflow            `json:"workflows"`
	Authors                []Author              `json:"authors"`
	CorrespondingAuthor    ExtendedAuthor        `json:"corresponding_author"`
	PrincipalInvestigators []ExtendedAuthor      `json:"principal_investigators"`
	WorkflowFile           WorkflowFile          `json:"workflow_file"`
	Imagesets              []Imageset            `json:"imagesets"`
	Citation               []Citation            `json:"citation"`
}

type CountryCode string

const (
	AD CountryCode = "AD"
	AE CountryCode = "AE"
	AF CountryCode = "AF"
	AG CountryCode = "AG"
	AI CountryCode = "AI"
	AL CountryCode = "AL"
	AM CountryCode = "AM"
	AO CountryCode = "AO"
	AQ CountryCode = "AQ"
	AR CountryCode = "AR"
	AS CountryCode = "AS"
	AT CountryCode = "AT"
	AU CountryCode = "AU"
	AW CountryCode = "AW"
	AZ CountryCode = "AZ"
	BA CountryCode = "BA"
	BB CountryCode = "BB"
	BD CountryCode = "BD"
	BE CountryCode = "BE"
	BF CountryCode = "BF"
	BG CountryCode = "BG"
	BH CountryCode = "BH"
	BI CountryCode = "BI"
	BJ CountryCode = "BJ"
	BL CountryCode = "BL"
	BM CountryCode = "BM"
	BN CountryCode = "BN"
	BO CountryCode = "BO"
	BR CountryCode = "BR"
	BS CountryCode = "BS"
	BT CountryCode = "BT"
	BV CountryCode = "BV"
	BW CountryCode = "BW"
	BY CountryCode = "BY"
	BZ CountryCode = "BZ"
	CA CountryCode = "CA"
	CC CountryCode = "CC"
	CD CountryCode = "CD"
	CF CountryCode = "CF"
	CG CountryCode = "CG"
	CH CountryCode = "CH"
	CI CountryCode = "CI"
	CK CountryCode = "CK"
	CL CountryCode = "CL"
	CM CountryCode = "CM"
	CN CountryCode = "CN"
	CO CountryCode = "CO"
	CR CountryCode = "CR"
	CU CountryCode = "CU"
	CV CountryCode = "CV"
	CW CountryCode = "CW"
	CX CountryCode = "CX"
	CY CountryCode = "CY"
	CZ CountryCode = "CZ"
	DE CountryCode = "DE"
	DJ CountryCode = "DJ"
	DK CountryCode = "DK"
	DM CountryCode = "DM"
	DO CountryCode = "DO"
	DZ CountryCode = "DZ"
	EC CountryCode = "EC"
	EE CountryCode = "EE"
	EG CountryCode = "EG"
	EH CountryCode = "EH"
	ER CountryCode = "ER"
	ES CountryCode = "ES"
	ET CountryCode = "ET"
	FI CountryCode = "FI"
	FJ CountryCode = "FJ"
	FK CountryCode = "FK"
	FM CountryCode = "FM"
	FO CountryCode = "FO"
	FR CountryCode = "FR"
	FX CountryCode = "FX"
	GA CountryCode = "GA"
	GB CountryCode = "GB"
	GD CountryCode = "GD"
	GE CountryCode = "GE"
	GF CountryCode = "GF"
	GG CountryCode = "GG"
	GH CountryCode = "GH"
	GI CountryCode = "GI"
	GL CountryCode = "GL"
	GM CountryCode = "GM"
	GN CountryCode = "GN"
	GP CountryCode = "GP"
	GQ CountryCode = "GQ"
	GR CountryCode = "GR"
	GS CountryCode = "GS"
	GT CountryCode = "GT"
	GU CountryCode = "GU"
	GW CountryCode = "GW"
	GY CountryCode = "GY"
	HK CountryCode = "HK"
	HM CountryCode = "HM"
	HN CountryCode = "HN"
	HR CountryCode = "HR"
	HT CountryCode = "HT"
	HU CountryCode = "HU"
	ID CountryCode = "ID"
	IE CountryCode = "IE"
	IL CountryCode = "IL"
	IM CountryCode = "IM"
	IN CountryCode = "IN"
	IO CountryCode = "IO"
	IQ CountryCode = "IQ"
	IR CountryCode = "IR"
	IS CountryCode = "IS"
	IT CountryCode = "IT"
	JE CountryCode = "JE"
	JM CountryCode = "JM"
	JO CountryCode = "JO"
	JP CountryCode = "JP"
	KE CountryCode = "KE"
	KG CountryCode = "KG"
	KH CountryCode = "KH"
	KI CountryCode = "KI"
	KM CountryCode = "KM"
	KN CountryCode = "KN"
	KP CountryCode = "KP"
	KR CountryCode = "KR"
	KW CountryCode = "KW"
	KY CountryCode = "KY"
	KZ CountryCode = "KZ"
	LA CountryCode = "LA"
	LB CountryCode = "LB"
	LC CountryCode = "LC"
	LI CountryCode = "LI"
	LK CountryCode = "LK"
	LR CountryCode = "LR"
	LS CountryCode = "LS"
	LT CountryCode = "LT"
	LU CountryCode = "LU"
	LV CountryCode = "LV"
	LY CountryCode = "LY"
	MA CountryCode = "MA"
	MC CountryCode = "MC"
	MD CountryCode = "MD"
	ME CountryCode = "ME"
	MF CountryCode = "MF"
	MG CountryCode = "MG"
	MH CountryCode = "MH"
	MK CountryCode = "MK"
	ML CountryCode = "ML"
	MM CountryCode = "MM"
	MN CountryCode = "MN"
	MO CountryCode = "MO"
	MP CountryCode = "MP"
	MQ CountryCode = "MQ"
	MR CountryCode = "MR"
	MS CountryCode = "MS"
	MT CountryCode = "MT"
	MU CountryCode = "MU"
	MV CountryCode = "MV"
	MW CountryCode = "MW"
	MX CountryCode = "MX"
	MY CountryCode = "MY"
	MZ CountryCode = "MZ"
	NA CountryCode = "NA"
	NC CountryCode = "NC"
	NE CountryCode = "NE"
	NF CountryCode = "NF"
	NG CountryCode = "NG"
	NI CountryCode = "NI"
	NL CountryCode = "NL"
	NO CountryCode = "NO"
	NP CountryCode = "NP"
	NR CountryCode = "NR"
	NU CountryCode = "NU"
	NZ CountryCode = "NZ"
	OM CountryCode = "OM"
	PA CountryCode = "PA"
	PE CountryCode = "PE"
	PF CountryCode = "PF"
	PG CountryCode = "PG"
	PH CountryCode = "PH"
	PK CountryCode = "PK"
	PL CountryCode = "PL"
	PM CountryCode = "PM"
	PN CountryCode = "PN"
	PR CountryCode = "PR"
	PS CountryCode = "PS"
	PT CountryCode = "PT"
	PW CountryCode = "PW"
	PY CountryCode = "PY"
	QA CountryCode = "QA"
)
