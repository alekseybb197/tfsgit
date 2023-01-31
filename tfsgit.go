package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"time"
	"net/url"
	"strings"

	confita "github.com/heetch/confita"
	confitaenv "github.com/heetch/confita/backend/env"
	confitafile "github.com/heetch/confita/backend/file"
	confitaflags "github.com/heetch/confita/backend/flags"

	"github.com/tidwall/gjson"
	"github.com/PuerkitoBio/goquery"

)

var version string

type Config struct {
	Cred   string  `config:"tfscred,short=c,required,description=user name and access token"`
	Repo   string  `config:"tfsrepo,short=r,required,description=repository url"`
	Branch string  `config:"tfsbranch,short=b,optional,description=branch name"`
	Path   string  `config:"tfspath,short=p,required,description=git path"`
	Depth  int     `config:"tfsdepth,short=d,optional,description=directory depth"`
	Quiet  bool    `config:"tfsquiet,short=q,optional,description=quiet mode"`
	Timeout  int   `config:"tfstimeout,short=t,optional,description=timeout secs"`
	Verbosity  int `config:"tfsverbosity,short=v,optional,description=output verbosity"`
}

// default values
var cfg = Config{
	Branch: "master",
	Depth:  10,
	Quiet:	false,
	Timeout: 5,
	Verbosity: 0, // it's not implemented yet
}

var tfsClient = http.Client{
	Timeout: time.Second * time.Duration(cfg.Timeout), // Timeout after N seconds
}

// depth level
var ndepth = 0

func tfsrequest(url string) *http.Response {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Set("User-Agent", "curl/7.79.1")
	req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(cfg.Cred)))
	//return req
	res, get := tfsClient.Do(req)
	if get != nil {
		log.Fatalln(get)
	}
	return res
}

func tfswalk(tfspath string) { // scan tfspath
	url := cfg.Repo + "/items?scopePath=" + tfspath + "/&recursionLevel=OneLevel&versionDescriptor.versionType=branch&version=" + url.QueryEscape(cfg.Branch)

	res := tfsrequest(url)
	if res.Body == nil {
		log.Println("Error -1")
		os.Exit(1)
	}
	defer res.Body.Close()

	// catch unknown code
	if res.StatusCode != 200 && res.StatusCode != 400 && res.StatusCode != 404 && res.StatusCode != 401{
        log.Fatalf("failed to fetch data: %s", res.Status)
    }

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatalln(readErr)
	}

	// fetch error from html title
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
    if err == nil {
        title := doc.Find("title").Text()
		if title != "" {
			log.Fatal(title)	
		}
    }

	// fetch error from json field message
    message := gjson.Get(string(body), "message")
	if message.String() != "" {
		log.Fatal(message.String())
	}

	// scan json when all conditions
	result := gjson.Get(string(body), "value")
	result.ForEach(func(key, value gjson.Result) bool {

		etype := gjson.Get(value.String(), "gitObjectType")
		epath := gjson.Get(value.String(), "path")
		eurl := gjson.Get(value.String(), "url")

		if etype.String() == "tree" {
			if epath.String() == tfspath { // ignore self
				return true // continue. if it is '.'
			} else { // subdir found!
				_, dirname := path.Split(epath.String())

				if _, err := os.Stat(dirname); errors.Is(err, os.ErrNotExist) {
					if !cfg.Quiet {
                        log.Println("make new directory -", dirname)
					}
					err := os.Mkdir(dirname, os.ModePerm)
					if err != nil {
						log.Println(err)
					}
				}

				if ndepth < cfg.Depth {
					cwd, _ := os.Getwd() // save current dir
					err := os.Chdir(dirname)
					if err != nil {
						log.Fatalln(err)
					} else {
						ndepth++
						tfswalk(epath.String())
						os.Chdir(cwd)
						ndepth--
					}
				}

				return true
			}
		}

		if etype.String() == "blob" { // get file
			_, filepath := path.Split(epath.String())
			if !cfg.Quiet {
				log.Println("download file -", filepath)
			}

			// need to change schema for download file because lfs
			pattern_path := regexp.MustCompile(`items//`)
			url := pattern_path.ReplaceAllString(eurl.String(),"items?path=")
			pattern_tail := regexp.MustCompile(`\?versionType`)
			url = pattern_tail.Split(url,-1)[0] + "&versionDescriptor%5BversionOptions%5D=0&versionDescriptor%5BversionType%5D=0&versionDescriptor%5Bversion%5D=" + cfg.Branch
			url = url + "&resolveLfs=true&api-version=5.0&download=true"

			res := tfsrequest(url)
			if res.Body != nil {
				defer res.Body.Close()
			}

			// Create the file
			out, createErr := os.Create(filepath)
			if createErr != nil {
				log.Fatalln(createErr)
			}
			defer out.Close()

			// Write the body to file
			_, copyErr := io.Copy(out, res.Body)
			if copyErr != nil {
				log.Fatalln(copyErr)
			}

			return true
		}

		// ignore unknown type
		log.Println("unknown type", etype.String(), "path", epath.String())

		return true // keep iterating
	})
}

func main() {
	// load actual values
	loader := confita.NewLoader(
		confitafile.NewOptionalBackend(".tfsgit.yaml"),
		confitaenv.NewBackend(),
		confitaflags.NewBackend(),
	)

	// process config error
	err := loader.Load(context.Background(), &cfg)
	if err != nil {
		log.Fatalln(err)
	}

	// supress lead slash if exists
	re, _ := regexp.Compile(`^/`)
	tfspath := "/" + re.ReplaceAllString(cfg.Path,"")

	if !cfg.Quiet {
		fmt.Printf("Version %+v\n", version)
		fmt.Println("Fetch " + tfspath)
	}

	tfswalk(tfspath)

}
