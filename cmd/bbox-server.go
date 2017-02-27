package main

/*
	This assumes:

	* Data that has been indexed by github:whosonfirst/go-whosonfirst-tile38/cmd/wof-tile38-index.go

	To do:

	* Handle cursors/pagination (from Tile38)
	* Use code on go-whosonfirst-tile38 (that still needs to be written) to convert Tile38
	  responses in to something a little friendlier and/or GeoJSON like...
	  
*/

import (
       "encoding/json"
	"flag"
	"fmt"
	"github.com/facebookgo/grace/gracehttp"
	"github.com/whosonfirst/go-whosonfirst-bbox/parser"	
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
)


type Tile38Coord struct {
     Latitude float64 `json:"lat"`
     Longitude float64 `json:"lon"`     
}

type Tile38Point struct {
     ID string `json:"id"`
     Point Tile38Coord `json:"point"`
     Fields []interface{} `json:"fields"`
}

type Tile38Response struct {
     Ok bool `json:"ok"`
     Count int `json:"count"`
     Cursor int `json:"cursor"`
     Fields []string `json:"fields"`
     Points []Tile38Point `json:"points"`
}

type WOFResponse struct {
     Results []WOFResult `json:"results"`
     Cursor  int `json:"cursor"`     
}

type WOFResult struct {
     WOFID     int64 `json:"wof:id"`
}

func main() {

	var host = flag.String("host", "localhost", "The hostname to listen for requests on")
	var port = flag.Int("port", 8080, "The port number to listen for requests on")

	var t38_host = flag.String("tile38-host", "127.0.0.1", "")
	var t38_port = flag.Int("tile38-port", 9851, "")
	var t38_collection = flag.String("tile38-collection", "dxlabs", "")

	flag.Parse()

	t38_addr := fmt.Sprintf("%s:%d", *t38_host, *t38_port)

	handler := func(rsp http.ResponseWriter, req *http.Request) {

		query := req.URL.Query()

		bbox := query.Get("bbox")
		scheme := query.Get("scheme")
		order := query.Get("order")
		
		// cursor := query.Get("cursor")		

		if bbox == "" {
			http.Error(rsp, "Missing bbox parameter", http.StatusBadRequest)
			return
		}

		p, err := parser.NewParser()

		if err != nil {
			http.Error(rsp, err.Error(), http.StatusInternalServerError)
			return
		}

		if scheme != "" {
			p.Scheme = scheme
		}

		if order != "" {
			p.Order = order
		}

		bb, err := p.Parse(bbox)

		if err != nil {
			http.Error(rsp, err.Error(), http.StatusInternalServerError)
			return
		}

		swlat := bb.MinY()
		swlon := bb.MinX()
		nelat := bb.MaxY()
		nelon := bb.MaxX()
		
		t38_cmd := fmt.Sprintf("INTERSECTS %s POINTS BOUNDS %0.6f %0.6f %0.6f %0.6f", *t38_collection, swlat, swlon, nelat, nelon)
		t38_url := fmt.Sprintf("http://%s/%s", t38_addr, url.QueryEscape(t38_cmd))

		log.Println(t38_url)

		t38_rsp, err := http.Get(t38_url)

		if err != nil {
			http.Error(rsp, err.Error(), http.StatusInternalServerError)
			return
		}

		defer t38_rsp.Body.Close()

		results, err := ioutil.ReadAll(t38_rsp.Body)

		if err != nil {
			http.Error(rsp, err.Error(), http.StatusInternalServerError)
			return
		}

		var r Tile38Response
		err = json.Unmarshal(results, &r)

		if err != nil {
			http.Error(rsp, err.Error(), http.StatusInternalServerError)
			return
		}

		wof_results := make([]WOFResult, 0)

		for _, p := range r.Points {
			wof_result := WOFResult{ int64(p.Fields[0].(float64)) }
			wof_results = append(wof_results, wof_result)
		}

		wof_response := WOFResponse{
			     Cursor: r.Cursor,
			     Results: wof_results,
		}

		b, _ := json.Marshal(wof_response)
		
		rsp.Header().Set("Access-Control-Allow-Origin", "*")
		rsp.Header().Set("Content-Type", "application/json")

		rsp.Write(b)
	}

	endpoint := fmt.Sprintf("%s:%d", *host, *port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)

	err := gracehttp.Serve(&http.Server{Addr: endpoint, Handler: mux})

	if err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}
