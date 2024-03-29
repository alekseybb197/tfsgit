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
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	confita "github.com/heetch/confita"
	confitaenv "github.com/heetch/confita/backend/env"
	confitafile "github.com/heetch/confita/backend/file"
	confitaflags "github.com/heetch/confita/backend/flags"

	"github.com/PuerkitoBio/goquery"
	"github.com/tidwall/gjson"
)

var version string

type Config struct {
	Cred      string `config:"tfscred,short=c,required,description=user name and access token"`
	Repo      string `config:"tfsrepo,short=r,required,description=repository url"`
	Branch    string `config:"tfsbranch,short=b,optional,description=branch name"`
	Match     string `config:"tfsmatch,short=m,optional,description=match name"`
	Path      string `config:"tfspath,short=p,required,description=git path"`
	Depth     int    `config:"tfsdepth,short=d,optional,description=directory depth"`
	Quiet     bool   `config:"tfsquiet,short=q,optional,description=quiet mode"`
	Timeout   int    `config:"tfstimeout,short=t,optional,description=timeout secs"`
	Verbosity int    `config:"tfsverbosity,short=v,optional,description=output verbosity"`
}

// default values
var cfg = Config{
	Branch:    "master",
	Match:     "",
	Depth:     10,
	Quiet:     false,
	Timeout:   5,
	Verbosity: 0,
}

var tfsClient = http.Client{
	Timeout: time.Second * time.Duration(cfg.Timeout), // Timeout after N seconds
}

// depth level
var ndepth = 0

// tfsrequest -- call TFS, return response
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

// tfswalk -- scan repo path
func tfswalk(tfspath string) bool { // scan tfspath

	url := cfg.Repo + "/items?scopePath=" + url.QueryEscape(tfspath) +
		"/&recursionLevel=OneLevel&versionDescriptor.versionType=branch&version=" + url.QueryEscape(cfg.Branch)
	if cfg.Verbosity > 0 {
		fmt.Printf("\nUrl %+v\n", url)
	}

	res := tfsrequest(url)
	if res.Body == nil {
		log.Println("Error -1")
		os.Exit(1)
	}
	defer res.Body.Close()

	// catch unknown code
	if res.StatusCode != 200 && res.StatusCode != 400 && res.StatusCode != 404 && res.StatusCode != 401 {
		log.Fatalf("failed to fetch data: %s", res.Status)
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatalln(readErr)
	}
	if cfg.Verbosity > 1 {
		fmt.Printf("\nResponse %+v\n", string(body))
	}

	// fetch error from html title
	doc, docErr := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if docErr != nil {
		log.Fatal(docErr)
	}

	title := doc.Find("title").Text()
	if title != "" {
		log.Fatal(title)
	}

	// fetch error from json field message
	message := gjson.Get(string(body), "message")
	if message.String() != "" {
		log.Fatal(message.String())
	}

	// scan json when all conditions
	result := gjson.Get(string(body), "value")
	if result.String() == "" {
		log.Fatal("api response not found")
	}

	if cfg.Verbosity > 1 {
		fmt.Printf("\nValue %+v\n", result)
	}

	// scan response json
	result.ForEach(func(key, value gjson.Result) bool {

		if cfg.Verbosity > 1 {
			fmt.Printf("\nScan %+v\n", value)
		}
		etype := gjson.Get(value.String(), "gitObjectType")
		epath := gjson.Get(value.String(), "path")
		eurl := gjson.Get(value.String(), "url")

		if etype.String() == "tree" {
			if cfg.Verbosity > 0 {
				fmt.Printf("\nFolder %+v\n", epath.String())
			}
			if epath.String() == tfspath || cfg.Match != "" { // ignore self or file search mode
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
			if cfg.Verbosity > 0 {
				fmt.Printf("\nFile %+v\n", epath.String())
			}
			if cfg.Match != "" { // download matched files only
				matched, merr := regexp.MatchString(cfg.Match, filepath)
				if merr != nil {
					log.Fatal(merr)
				}
				if !matched {
					return true
				}
			}

			if !cfg.Quiet {
				log.Println("download file -", filepath)
			}

			// need to change schema for download file because lfs
			pattern_path := regexp.MustCompile(`items//`)
			url := pattern_path.ReplaceAllString(eurl.String(), "items?path=")
			pattern_tail := regexp.MustCompile(`\?versionType`)
			url = pattern_tail.Split(url, -1)[0] +
				"&versionDescriptor%5BversionOptions%5D=0&versionDescriptor%5BversionType%5D=0&versionDescriptor%5Bversion%5D=" +
				cfg.Branch
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

	return true
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

	// suppress lead and tail slash if exists
	leadslash, _ := regexp.Compile(`^/`) // drop lead slash
	tailslash, _ := regexp.Compile(`/$`) // drop tail slash
	tfspath := "/" + leadslash.ReplaceAllString(tailslash.ReplaceAllString(cfg.Path, ""), "")

	if cfg.Match != "" { // search files into root level only!
		cfg.Depth = 0
	}

	if !cfg.Quiet {
		fmt.Printf("Version %+v\n", version)
		fmt.Println("Fetch " + tfspath)
		if cfg.Match != "" {
			fmt.Println("Match " + cfg.Match)
		}
	}

	// go ahead
	tfswalk(tfspath)

}
