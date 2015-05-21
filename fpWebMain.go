// fpWebMain
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
)

var buildDate string

func main() {
	log.Println(buildDate)

	log.Println("Generating Blog ")
	GenerateBlog()

	log.Println("Generating Hobby ")
	GenerateHobby()

	log.Println("Generating Sitemap ")
	GenerateSiteMap()

	go WebServeStaticFolder(":1667", ".")

	text := ""
	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter text: ")
		text, _ = reader.ReadString('\n')
		fmt.Println(text)

		switch text[0] {
		case 'x':
			log.Fatalln("Exit")
		case 'r':
			log.Println("Generating Pages ")
			GenerateBlog()

			log.Println("Generating Sitemap ")
			GenerateSiteMap()
		}
	}
}
