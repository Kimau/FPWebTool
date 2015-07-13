package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type WebFace struct {
	Addr   string
	Router *http.ServeMux

	OutMsg             chan string
	InMsg              chan string
	GlobalTemplateData map[string]string
}

func MakeWebFace(addr string, hostfileroot string) *WebFace {
	w := &WebFace{
		Addr:   addr,
		Router: http.NewServeMux(),

		OutMsg:             make(chan string),
		InMsg:              make(chan string),
		GlobalTemplateData: make(map[string]string),
	}

	w.MakeTemplates()

	w.Router.HandleFunc("/admin/blog/list", w.ServeBlogList)
	w.Router.HandleFunc("/admin/blog/", w.ServeBlogPage)
	w.Router.HandleFunc("/admin/generate", w.ServeGenerate)
	w.Router.HandleFunc("/admin/", w.ServeAdminPage)
	w.Router.Handle("/", http.FileServer(http.Dir(hostfileroot)))

	go w.HostLoop()

	return w
}

// TEMP HACK
var AdminTemplate, EditTemplate, ListTemplate *template.Template

func (wf *WebFace) MakeTemplates() {
	var err error
	ListTemplate, err = template.New("list").Parse(`<!DOCTYPE html>
<html>
<head>
  <title>Blog Listing</title>
  <style>
  table {border-spacing: 0px;
    border-collapse: separate;}
  td {  margin: 0;  padding: 0;    border-spacing: 0px;   border-color: #888;   border-width: 1px;   border-style: solid;  }
  tr { margin: 0; padding: 0; border-bottom: 3px #000 solid; }
  </style>
</head>
<body>

<table>
{{range .}}
<tr>
<td><a href="/admin/blog/{{.Key}}/edit">{{.Title}}</a>
<div>
{{range .RawCategory}}
<span>#{{.}}</span>
{{end}}
</div>
</td>
<td>{{printf "%04d-%02d-%02d %02d:%02d" .Date.Year .Date.Month .Date.Day .Date.Hour .Date.Minute}}</td>
{{if .SmallImage}}
<td style="background: url('{{.SmallImage}}') no-repeat 0 20px;vertical-align: top;background-size: contain; width: 120px; height: 120px;"></td>
{{else}}
<td>X</td>
{{end}}

{{if .BannerImage}}
<td style="background: url('{{.BannerImage}}') no-repeat 0 20px;vertical-align: top;background-size: contain; width: 250px; height: 100px;"></td>
{{else}}
<td>X</td>
{{end}}

<td style="width: 300px;">{{.ShortDesc}}</td>
</tr>
{{end}}
</table>

</body>
</html>`)

	if err != nil {
		log.Fatalln(err)
	}

	EditTemplate, err = template.New("edit").Parse(`<!DOCTYPE html>
<html>
<head>
  <title>Editing {{.Key}}</title>
</head>
<body>

<div style="width:48%; display:inline-block; vertical-align:top;">
<h1>{{.Key}}</h1>
 <form action="/admin/blog/{{.Key}}/save" method="POST">
  <div><input name="Title" type="text" value="{{.Title}}"></input>
  <input name="RawCategory" type="text" value="{{range .RawCategory}}{{.}},{{end}}"></input>
  </div><div><input name="PubdateDate" type="date" value="{{printf "%04d-%02d-%02d" .Date.Year .Date.Month .Date.Day}}"></input>
  <input name="PubdateTime" type="time" value="{{printf "%02d:%02d" .Date.Hour .Date.Minute}}"></input>
  </div><div>{{if .SmallImage}}<img src="{{.SmallImage}}"/> {{end}} <input name="SmallImage" type="text" value="{{.SmallImage}}"></input>
  </div><div>{{if .BannerImage}}<img src="{{.BannerImage}}"/> {{end}} <input name="BannerImage" type="text" value="{{.BannerImage}}"></input>
  </div>
  <textarea spellcheck="true" name="ShortDesc" style="width:90%; height:150px;">{{.ShortDesc}}</textarea>
  <textarea spellcheck="true" name="Body" style="width:90%; height:600px;">{{.Body}}</textarea>
  <input type="submit" value="Save">
 </form>

 </div>

        <iframe width="48%" height="1600px" style="width:48%; display:inline-block;" src="/{{.Link}}" frameborder="0" allowfullscreen></iframe>
</body>
</html>`)

	if err != nil {
		log.Fatalln(err)
	}

	AdminTemplate, err = template.New("admin").Parse(`<!DOCTYPE html>
<html>
<head>
  <title>Admin</title>
</head>
<body>
 <div><a href="/admin/generate">Generate Webpage</a></div>
 <div><a href="/admin/blog/list">Blog Listing</a></div>
  {{range .}}
 <div>{{.}}</div>
 {{end}}
</body>
</html>`)

	if err != nil {
		log.Fatalln(err)
	}
	//
}

var validBlogPath = regexp.MustCompile("^/admin/blog/([a-zA-Z0-9\\-]+)/(edit|save|view)$")

func (wf *WebFace) ServeBlogPage(w http.ResponseWriter, req *http.Request) {
	m := validBlogPath.FindStringSubmatch(req.URL.Path)

	// Get Page
	b := myData.Feed.Get(m[1])

	switch m[2] {
	case "edit":
		err := EditTemplate.ExecuteTemplate(w, "edit", b)
		if err != nil {
			log.Fatalln(err)
		}

	case "save":
		b.Title = req.FormValue("Title")
		catList := strings.Split(strings.TrimRight(req.FormValue("RawCategory"), " ,"), ",")
		b.RawCategory = []BlogCat{}
		for _, v := range catList {
			v = strings.Trim(v, " ,#")
			b.RawCategory = append(b.RawCategory, BlogCat(v))
		}

		// Set Pub Date
		var year, month, day, hour, min int
		fmt.Sscanf(req.FormValue("PubdateDate"), "%04d-%02d-%02d", &year, &month, &day)
		fmt.Sscanf(req.FormValue("PubdateTime"), "%02d:%02d", &hour, &min)
		log.Printf("%s \n %s \n %04d-%02d-%02d %02d:%02d", req.FormValue("PubdateDate"), req.FormValue("PubdateTime"), year, month, day, hour, min)
		b.SetNewPubDate(time.Date(year, time.Month(month), day, hour, min, 0, 0, time.UTC))

		// Setup Images
		b.SmallImage = req.FormValue("SmallImage")
		b.BannerImage = req.FormValue("BannerImage")
		b.ShortDesc = req.FormValue("ShortDesc")
		b.Body = template.HTML(req.FormValue("Body"))

		myData.Feed.SaveToFile()
		b.GeneratePage()

		http.Redirect(w, req, "/admin/blog/"+m[1]+"/edit", 302)

	case "view":
		fmt.Fprint(w, "Big TODO okay")
	default:
		fmt.Fprint(w, "WTF Chick!?")
	}
}

func (wf *WebFace) ServeGenerate(w http.ResponseWriter, req *http.Request) {
	wf.InMsg <- "generate"
	wf.GlobalTemplateData["isGenerating"] = "generating"
	http.Redirect(w, req, "/admin/", 302)
}

func (wf *WebFace) ServeAdminPage(w http.ResponseWriter, req *http.Request) {

	err := AdminTemplate.ExecuteTemplate(w, "admin", wf.GlobalTemplateData)
	if err != nil {
		log.Fatalln(err)
	}
}

func (wf *WebFace) ServeBlogList(w http.ResponseWriter, req *http.Request) {

	err := ListTemplate.ExecuteTemplate(w, "list", myData.Feed)
	if err != nil {
		log.Fatalln(err)
	}
}

func (wf *WebFace) HostLoop() {
	defer log.Println("Stopped Listening")

	log.Println("Listening on " + wf.Addr)
	err := http.ListenAndServe(wf.Addr, wf.Router)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

//===
