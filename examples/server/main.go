package main

import (
	"embed"
	"flag"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"sync"

	"go.hacdias.com/indiekit/indieauth"
)

func main() {
	// Setup flags.
	portPtr := flag.Int("port", 80, "port to listen on")
	addressPtr := flag.String("profile", "http://localhost/", "client URL and front facing address to listen on")
	flag.Parse()

	profileURL := *addressPtr

	// Validate the given Client ID before starting the HTTP server.
	err := indieauth.IsValidProfileURL(profileURL)
	if err != nil {
		log.Fatal(err)
	}

	// Create a new client.
	server := &server{
		profileURL:     profileURL,
		authorizations: map[string]*authorization{},
		ias:            indieauth.NewServer(true, nil),
	}

	http.HandleFunc("/", server.indexHandler)
	http.HandleFunc("/authorization", server.authorizationHandler)
	http.HandleFunc("/authorization/accept", server.authorizationAcceptHandler)
	// Note: in production servers, the token endpoint, as well as the token
	// verification endpoint should be also implemented. The token endpoint
	// would be identical to the authorizationAcceptHandler, except that
	// it would also produce a token as per the spec. I would recommend checking
	// IndieAuth and OAuth2 specifications for more details.

	log.Printf("Listening on http://localhost:%d", *portPtr)
	log.Printf("Listening on %s", profileURL)
	if err := http.ListenAndServe(":"+strconv.Itoa(*portPtr), nil); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	profileURL       string
	authorizations   map[string]*authorization
	authorizationsMu sync.Mutex
	ias              *indieauth.Server
}

var (
	//go:embed templates/*.html
	templatesFs embed.FS

	templates = template.Must(template.ParseFS(templatesFs, "templates/*.html"))
)

// indexHandler serves a simple index page with a login form.
func (s *server) indexHandler(w http.ResponseWriter, r *http.Request) {
	// Advertise authorization endpoint. There are multiple ways of doing this.
	w.Header().Set("Link", `</authorization>; rel="authorization_endpoint"`)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.ExecuteTemplate(w, "index.html", map[string]string{"Profile": s.profileURL})
}
