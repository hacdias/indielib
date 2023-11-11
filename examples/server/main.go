package main

import (
	"embed"
	"encoding/json"
	"flag"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"sync"

	"go.hacdias.com/indiekit/indieauth"
	"go.hacdias.com/indiekit/micropub"
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
	s := &server{
		profileURL:     profileURL,
		authorizations: map[string]*authorization{},
		tokens:         map[string]*token{},
		posts:          map[string]post{},
		ias:            indieauth.NewServer(true, nil),
	}

	// Mount general handler, which will handle the index page, as well as the
	// post pages.
	http.HandleFunc("/", s.generalHandler)

	// Mounts the IndieAuth-related handlers. Since IndieAuth is an extension of OAuth2,
	// it is important to be familiarized with how OAuth2 works. In addition, it is
	// important to mention that not all OAuth2 handlers have been implemented.
	http.HandleFunc("/authorization", s.authorizationHandler)
	http.HandleFunc("/authorization/accept", s.authorizationAcceptHandler)
	http.HandleFunc("/token", s.tokenHandler)

	// Mounts the Micropub handler. We don't send any special configuration besides our
	// implementation. Note that we wrap it with [server.mustAuth] which ensures that
	// only authenticated requests pass through.
	http.Handle("/micropub", s.mustAuth(micropub.NewHandler(&micropubImplementation{s})))

	// Start it!
	log.Printf("Listening on http://localhost:%d", *portPtr)
	log.Printf("Listening on %s", profileURL)
	if err := http.ListenAndServe(":"+strconv.Itoa(*portPtr), nil); err != nil {
		log.Fatal(err)
	}
}

type post struct {
	Type       string
	Properties map[string][]any
}

type server struct {
	profileURL       string
	authorizations   map[string]*authorization
	authorizationsMu sync.Mutex
	tokens           map[string]*token
	tokensMu         sync.Mutex
	posts            map[string]post
	postsMu          sync.RWMutex
	ias              *indieauth.Server
}

var (
	//go:embed templates/*.html
	templatesFs embed.FS

	templates = template.Must(template.ParseFS(templatesFs, "templates/*.html"))
)

func (s *server) generalHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		serveHTML(w, "index.html", map[string]any{
			"Profile": s.profileURL,
			"Posts":   s.posts,
		})
		return
	}

	s.postsMu.RLock()
	defer s.postsMu.RUnlock()
	if post, ok := s.posts[r.URL.Path]; ok {
		serveHTML(w, "post.html", post)
		return
	}

	httpError(w, http.StatusNotFound)
}

func httpError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

func serveHTML(w http.ResponseWriter, template string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.ExecuteTemplate(w, template, data)
}

func serveJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

func serveErrorJSON(w http.ResponseWriter, code int, err, errDescription string) {
	serveJSON(w, code, map[string]string{
		"error":             err,
		"error_description": errDescription,
	})
}
