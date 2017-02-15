package main

import (
	"flag"
	"fmt"
	"github.com/facebookgo/grace/gracehttp"
	"github.com/thisisaaronland/go-marc/fields"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

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

		str_bbox := query.Get("bbox")
		str_marc := query.Get("marc")

		if str_bbox == "" && str_marc == "" {
			http.Error(rsp, "Missing bbox or marc parameter", http.StatusBadRequest)
			return
		}

		var swlat float64
		var swlon float64
		var nelat float64
		var nelon float64
		var err error

		if str_bbox != "" {

			parts := strings.Split(str_bbox, ",")

			swlat, err = strconv.ParseFloat(parts[0], 64)

			if err != nil {
				http.Error(rsp, "Invalid SW latitude parameter", http.StatusBadRequest)
				return
			}

			swlon, err = strconv.ParseFloat(parts[1], 64)

			if err != nil {
				http.Error(rsp, "Invalid SW longitude parameter", http.StatusBadRequest)
				return
			}

			nelat, err = strconv.ParseFloat(parts[2], 64)

			if err != nil {
				http.Error(rsp, "Invalid NE latitude parameter", http.StatusBadRequest)
				return
			}

			nelon, err = strconv.ParseFloat(parts[3], 64)

			if err != nil {
				http.Error(rsp, "Invalid NE longitude parameter", http.StatusBadRequest)
				return
			}

			if swlat > 90.0 || swlat < -90.0 {
				http.Error(rsp, "E_IMPOSSIBLE_LATITUDE (SW)", http.StatusBadRequest)
				return
			}

			if nelat > 90.0 || nelat < -90.0 {
				http.Error(rsp, "E_IMPOSSIBLE_LATITUDE (NE)", http.StatusBadRequest)
				return
			}

			if swlon > 180.0 || swlon < -180.0 {
				http.Error(rsp, "E_IMPOSSIBLE_LONGITUDE (SW)", http.StatusBadRequest)
				return
			}

			if nelon > 180.0 || nelon < -180.0 {
				http.Error(rsp, "E_IMPOSSIBLE_LONGITUDE (ne)", http.StatusBadRequest)
				return
			}

		} else if str_marc != "" {

			parsed, err := fields.Parse034(str_marc)

			if err != nil {
				http.Error(rsp, "Invalid 034 MARC string", http.StatusBadRequest)
				return
			}

			bbox, err := parsed.BoundingBox()

			if err != nil {
				http.Error(rsp, "Failed to derive bounding box from 034 MARC string", http.StatusBadRequest)
				return
			}

			swlat = bbox.SW.Latitude
			swlon = bbox.SW.Longitude
			nelat = bbox.NE.Latitude
			nelon = bbox.NE.Longitude

		} else {
			// pass
		}

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

		rsp.Header().Set("Access-Control-Allow-Origin", "*")
		rsp.Header().Set("Content-Type", "application/json")

		rsp.Write(results)
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
