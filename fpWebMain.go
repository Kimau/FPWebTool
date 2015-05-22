// fpWebMain
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
)

var buildDate string

func genWebsite() {
	log.Println("Generating Blog ")
	GenerateBlog()

	log.Println("Generating Hobby ")
	GenerateHobby()

	log.Println("Generating Job ")
	GenerateJob()

	log.Println("Generating About ")
	GenerateAbout()

	log.Println("Generating Sitemap ")
	GenerateSiteMap()
}

func main() {
	log.Println(buildDate)

	genWebsite()

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
			genWebsite()
		}
	}
}
