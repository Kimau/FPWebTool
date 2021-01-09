// fpWebMain
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

var (
	buildDate string
)

const publicHtmlRoot = "./public_html/"

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

func processCommand(line string, wf *WebFace) {
	// fmt.Println(line)

	switch line {
	case "x", "exit":
		log.Fatalln("Exit")
	case "g", "generate":
		Generate()
		wf.GlobalTemplateData["isGenerating"] = "Done"
	default:
		fmt.Println("Commands: " + strings.Join([]string{"g", "generate", "x", "exit"}, " "))
	}
}

func copyFolderOver(folder string, destFolder string, c chan (int)) {
	err := CopyTree("./"+folder, publicHtmlRoot+destFolder, false)

	if err != nil {
		log.Fatalln("Failed to copy " + folder + " because " + err.Error())
	}

	log.Printf("Copied %s to web root\n", folder)
	c <- 1
}

func Generate() {
	os.RemoveAll(publicHtmlRoot)
	c1 := make(chan int)
	c2 := make(chan int)
	go copyFolderOver("static_folder", "", c1)
	go copyFolderOver("images", "images", c2)

	genWebsite()

	// wait on gen
	log.Println("----------------------------------------------\n Waiting on file copies...")
	<-c1
	<-c2
}

func main() {
	flagGenSite := flag.Bool("gen", false, "Should Website be generated")
	microGenSite := flag.Bool("micro", false, "Should Micro be generated")
	flag.Parse()

	log.Println(buildDate)

	if *flagGenSite {
		Generate()
	} else if *microGenSite {
		generateDataOnly() // only
		setupRoot()
		GenerateMicro()
	} else {
		generateDataOnly() // only
	}

	wf := MakeWebFace(":1667", publicHtmlRoot)
	lines := scanForInput()

	for {
		fmt.Println("Enter Command: ")
		select {
		case line := <-lines:
			processCommand(line, wf)
		case m := <-wf.InMsg:
			processCommand(m, wf)
		}

	}
}
