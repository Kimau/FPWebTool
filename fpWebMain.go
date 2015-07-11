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

func scanForInput() chan string {
	lines := make(chan string)

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	return lines
}

func processCommand(line string) {
	fmt.Println(line)
	switch line[0] {
	case 'x':
		log.Fatalln("Exit")
	case 'r':
		genWebsite()
	}

}

func main() {
	log.Println(buildDate)

	genWebsite()

	wf := MakeWebFace(":1667", ".")
	lines := scanForInput()

	for {
		fmt.Println("Enter Command: ")
		select {
		case line := <-lines:
			processCommand(line)
		case m := <-wf.InMsg:
			log.Println(m)
		}

	}
}
