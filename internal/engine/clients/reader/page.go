package reader

type Page struct {
	Items      [][]byte
	Size       int
	Number     int
	TotalPages int
	TotalItems int
}
