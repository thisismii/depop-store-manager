package main

import (
	"flag"
	"fmt"
	"os"

	"depop-store-manager/internal/store"
)

// Confirmed books-review force links (U gaps). U4/U6/U18 intentionally omitted.
var reviewLinks = []store.BooksLink{
	{ProductID: 495886373, Item: "Tan Adidas Sambas"},
	{ProductID: 490608023, Item: "Black YouTube logo hat"},
	{ProductID: 506861984, Item: "New Orleans Hat"},
	{ProductID: 512650500, Item: "ASOS brown handbag"},
	{ProductID: 512642779, Item: "cream ribbed keyhole tank croptop"},
	{ProductID: 512644428, Item: "brown collared sleeveless top"},
	{ProductID: 506873645, Item: "Abercrombie light blue oversized tee shirt A&F essentials"},
	{ProductID: 570717532, Item: "ADIDAS Grey and black athletic shorts"},
	{ProductID: 506920953, Item: "maxtop waterproof fannypack black"},
	{ProductID: 494754963, Item: "Green long sleeve thermal shirt with snap up enclosure"},
	{ProductID: 490604705, Item: "Budweiser baseball cap"},
	{ProductID: 506573508, Item: "ZARA black halted pleat dress short"},
	{ProductID: 506865138, Item: "RHONE tan weekender sweatpants"},
	{ProductID: 645194804, Item: "TRIANGL bikini set - blue, white, highlighter yellow"},
	{ProductID: 691625862, Item: "REFORMATION Women's Black Bodycon-dresses"},
}

func main() {
	csvPath := flag.String("csv", "data/books-seed.csv", "path to books CSV")
	catalogPath := flag.String("catalog", "data/catalog.json", "path to catalog.json")
	flag.Parse()

	f, err := os.Open(*csvPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	rows, err := store.ReadBooksCSV(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cs := store.NewCatalogStore(*catalogPath)
	res, err := cs.ApplyBooksLinks(rows, reviewLinks)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("links=%d matched=%d applied=%d skipped=%d\n",
		len(reviewLinks), res.Matched, res.Applied, res.Skipped)
	for _, w := range res.Weak {
		fmt.Println(" ", w)
	}
}
