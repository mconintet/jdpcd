package main

import (
	"net/http"
	"bytes"
	"fmt"
	"errors"
	"time"
	"strconv"
	"net/url"
	"io"
	"net/http/cookiejar"
	"github.com/mconintet/conv"
	"github.com/mconintet/clicolor"
	"bufio"
	"encoding/json"
	"io/ioutil"
	"flag"
	"log"
)

type District struct {
	Id   string
	Name string
}

type City struct {
	Id       string
	Name     string
	Children []District
}

type Province struct {
	Id       string
	Name     string
	Children []City
}

func beautifyAgent(req *http.Request) {
	if req.Header == nil {
		req.Header = make(http.Header)
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_9_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2272.76 Safari/537.36")
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Add("Accept-Encoding", "gzip, deflate, sdch")
}

// Columns of raw cookie data:
// domain - The domain that created AND that can read the variable. 
// flag - A TRUE/FALSE value indicating if all machines within a given domain can access the variable.  Say "true"
// path - The path within the domain that the variable is valid for.  Use / for any url
// secure - A TRUE/FALSE value indicating if a secure connection with the domain is needed to access the variable. Use false to allow http://
// expiration - The UNIX time that the variable will expire on.  Set something far in the future
// name - The name of the variable.
// value - The value of the variable.
func loadCookies(jar http.CookieJar, u *url.URL, raw *bytes.Buffer) error {
	var (
		line []byte
		err error
		cols [][]byte
		cookies []*http.Cookie
		cookie *http.Cookie
		dp [][]byte
		sec int64
		nsec int64
		rawExp string
	)

	cookies = []*http.Cookie{}

	for {
		if line, err = raw.ReadBytes('\n'); err != nil {
			if err == io.EOF {
				break;
			}else {
				return err
			}
		}

		line = bytes.TrimRight(line, "\n")

		cols = bytes.Split(line, []byte("\t"));
		if len(cols) != 7 {
			continue
		}

		cookie = new(http.Cookie)
		cookie.Domain = string(cols[0])
		cookie.Path = string(cols[2])
		cookie.Secure = string(cols[3]) == "TRUE"

		dp = bytes.Split(cols[4], []byte("."))
		switch len(dp){
		case 1:
			rawExp = string(dp[0])

			if sec, err = strconv.ParseInt(rawExp, 10, 64); err != nil {
				return err
			}

			if sec == 0 {
				cookie.Expires = time.Time{}
			}else {
				cookie.Expires = time.Unix(sec, 0)
			}

			cookie.RawExpires = rawExp
		case 2:
			rawExp = string(dp[0])+"."+string(dp[1])

			if sec, err = strconv.ParseInt(string(dp[0]), 10, 64); err != nil {
				return err
			}

			if nsec, err = strconv.ParseInt(string(dp[1]), 10, 64); err != nil {
				return err
			}

			cookie.Expires = time.Unix(sec, nsec)
			cookie.RawExpires = rawExp
		default:
			return errors.New("unrecognized format of expiration date.")
		}

		cookie.Name = string(cols[5])
		cookie.Value = string(cols[6])

		cookies = append(cookies, cookie)
	}

	jar.SetCookies(u, cookies)
	return nil
}

var cookies string

func getJson(req *http.Request) (jsonData map[string]string) {
	cookiesBuf := bytes.NewBuffer([]byte(cookies))
	jar, _ := cookiejar.New(nil)
	client := &http.Client{}
	client.Jar = jar

	loadCookies(jar, req.URL, cookiesBuf)

	beautifyAgent(req)
	resp, _ := client.Do(req)

	// data from jd is encoded by GBK, so need to do a encoding converting here
	var out bytes.Buffer
	if err := conv.GbkToUtf8(bufio.NewReader(resp.Body), &out, true); err != nil {
		panic(err)
	}

	if err := json.Unmarshal(out.Bytes(), &jsonData); err != nil {
		respContent, _ := ioutil.ReadAll(resp.Body)
		panic(errors.New(fmt.Sprintf("error: %s got: %s url: %s", err.Error(), respContent, req.URL.String())))
	}

	return jsonData
}

func toProvinces(json map[string]string) []Province {
	var ps []Province
	for k, v := range json {
		p := Province{
			k,
			v,
			nil,
		}

		ps = append(ps, p)
	}

	return ps
}

func toCities(json map[string]string) []City {
	var cs []City
	for k, v := range json {
		c := City{
			k,
			v,
			nil,
		}

		cs = append(cs, c)
	}

	return cs
}

func toDistricts(json map[string]string) []District {
	var ds []District
	for k, v := range json {
		d := District{
			k,
			v,
		}

		ds = append(ds, d)
	}

	return ds
}

func grab() ([]Province) {
	provincesUrl := "http://easybuy.jd.com/address/getProvinces.action"
	citiesUrl := "http://easybuy.jd.com/address/getCitys.action"
	districtsUrl := "http://easybuy.jd.com/address/getCountys.action"

	req, _ := http.NewRequest("GET", provincesUrl, bytes.NewBuffer([]byte{}))
	provincesJson := getJson(req)

	provinces := toProvinces(provincesJson)
	for pk, p := range provinces {
		// province with id 84 is a trap, there is no city of "钓鱼岛" orz...
		if p.Id == "84" {
			continue
		}

		str, _ := clicolor.ColorizeStr("--> Getting cities of "+p.Name+"...", "white", "blue")
		fmt.Println(str)

		req, _ = http.NewRequest("GET", citiesUrl+"?provinceId="+p.Id, bytes.NewBuffer([]byte{}))
		time.Sleep(time.Millisecond * 300)
		citiesJson := getJson(req)

		p.Children = toCities(citiesJson)

		for ck, c := range p.Children {
			fmt.Println("Getting districts of " + c.Name + "...")

			req, _ = http.NewRequest("GET", districtsUrl+"?cityId="+c.Id, bytes.NewBuffer([]byte{}))
			time.Sleep(time.Millisecond * 300)
			districtsJson := getJson(req)

			c.Children = toDistricts(districtsJson)
			p.Children[ck] = c
		}

		provinces[pk] = p
	}

	return provinces
}

func main() {
	var (
		cookiesFilePath string
		err error
		outFilePath string
		outBytes []byte
		cookiesBytes []byte
	)

	flag.StringVar(&cookiesFilePath, "c", "", "cookies file")
	flag.StringVar(&outFilePath, "o", "", "output file")

	flag.Parse()

	if cookiesFilePath == "" || outFilePath == "" {
		flag.Usage()
		log.Fatal("arguments error")
	}

	if cookiesBytes, err = ioutil.ReadFile(cookiesFilePath); err != nil {
		log.Fatal(err)
	}

	cookies = string(cookiesBytes)

	data := grab()
	if outBytes, err = json.Marshal(data); err != nil {
		log.Fatal(err)
	}

	if err = ioutil.WriteFile(outFilePath, outBytes, 0666); err != nil {
		log.Fatal(err)
	}

	str, _ := clicolor.ColorizeStr("Done!", "green", "black")
	fmt.Println(str)
}
