# OpenEM Depositor of EM data to International Repositories

This repository provides a depositor app backend on top of SciCat frontend that allows users to seamlessly  deposit their data to OneDep system. OneDep covers depositions to PDB end EMDB.  

This depositor is designed for Swiss OpenEm project and supports only depositions to OneDep that are experimentally based on methods covered by Electron Microscopes. 

### Files necessary for a deposition:
OneDep defines an obligatory set of files required to create a deposition based on the specific method. Subsequently, same files are necessary for this depositor too.  
Additionally, the user can provide an extra file with metadata that will be accepted by the depositor and appended to the deposition. This metadata file must be in json format and follow the [OSC-EM schema](https://github.com/osc-em/OSCEM_Schemas) supported by OpenEM. Once provided, this metadata will be automatically [converted to mmCIF format](https://github.com/osc-em/converter-OSCEM-to-mmCIF) accepted by OneDep. If deposition is destined to PDB, a new mmCIF file will be created containing both metadata and coordinates model provided by the user. For EMDB depositions a brand new mmCIF file will be created.  

In OneDep user needs to manually provide the pixel spacing. Here user is not required to do it, as the [headers definition](https://bio3d.colorado.edu/imod/betaDoc/mrc_format.txt) of _.mrc_ files provide this information. 
`pixel_spacing_{x/y/z} = cell_dimensions_{x/y/z} / sampling_unit_cell_{x/y/z}`

## Limitations
* `depositions/onedep/config.go` defined the base URL, to which requests will be sent to create deposition, add files and files metadata. Currently only a [test server](https://onedep-depui-test.wwpdb.org/) for OneDep is used. Once, the API is rolled out in the production (https://deposit-pdbe.wwpdb.org/deposition/ in Europe), this will then be changed. 
* Additional Endpoint will be implemented to create deposition in EMPIAR for raw datasets. This is expected by March 2025, as described on our [Project Timeline](https://swissopenem.github.io/timeline/).
* Authorization with ORCID Id is not supported yet, requires a jwt token that needs to be created by user.
* currently configured to run on localhost:8080 
