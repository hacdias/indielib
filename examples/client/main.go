package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"text/template"
	"time"

	"github.com/hacdias/indieauth"
)

const (
	oauthCookieName string = "indieauth-cookie"
)

var (
	indexTemplate = `<!DOCTYPE html>
	<html>
		<head>
			<title>IndieAuth Client Demo</title>
		</head>
		<body>
			<h1>IndieAuth Client Demo</h1>

			<p>Sign in with your domain:</p>
			
			<form action="/login" method="post">
				<input type="text" name="profile" placeholder="yourdomain.com" required />
				<button/>Sign In</button>
			</form>
		</body>
	</html>`

	loggedInTemplate = template.Must(template.New("").Parse(`
	<!DOCTYPE html>
	<html>
		<head>
			<title>You are logged in!</title>
		</head>
		<body>
			<h1>Welcome {{ .Me }}!</h1>

			<p>You are now successfully logged in. This is what we gathered about you:</p>

			<ul>
				<li>Name: {{ or .Profile.Name "unknown" }}</li>
				<li>URL: {{ or .Profile.URL "unknown" }}</li>
				<li>Photo: {{ or .Profile.Photo "unknown" }}</li>
				<li>E-mail: {{ or .Profile.Email "unknown" }}</li>
			</ul>
		</body>
	</html>
	`))
)

func main() {
	// Setup flags.
	portPtr := flag.Int("port", 3535, "port to listen on")
	addressPtr := flag.String("client", "http://localhost:3535/", "client ID, front facing address to listen on")
	flag.Parse()

	clientID := *addressPtr
	callbackURI := clientID + "callback"

	// Validate the given Client ID before starting the HTTP server.
	err := indieauth.IsValidClientIdentifier(clientID)
	if err != nil {
		log.Fatal(err)
	}

	// Create a new client.
	client := &client{
		iac: indieauth.NewClient(clientID, callbackURI, nil),
	}

	http.HandleFunc("/", client.indexHandler)
	http.HandleFunc("/login", client.loginHandler)
	http.HandleFunc("/callback", client.callbackHandler)

	log.Printf("Listening on http://localhost:%d", *portPtr)
	log.Printf("Listening on %s", clientID)
	if err := http.ListenAndServe(":"+strconv.Itoa(*portPtr), nil); err != nil {
		log.Fatal(err)
	}
}

type client struct {
	iac *indieauth.Client
}

// indexHandler serves a simple index page with a login form.
func (s *client) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexTemplate))
}

// loginHandler handles the login process after submitting the domain via the
// index page.
func (s *client) loginHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	profileURL := r.FormValue("profile")
	if profileURL == "" {
		http.Error(w, "empty profile", http.StatusBadRequest)
		return
	}

	// After retrieving the profile URL from the login form, canonicalize it,
	// and check if it is valid.
	profileURL = indieauth.CanonicalizeURL(profileURL)
	if err := indieauth.IsValidProfileURL(profileURL); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generates the redirect request to the target profile so that the user can
	// authorize the request. We also ask for the "profile" and "email" scope so
	// that we can get more information about the user.
	authInfo, redirect, err := s.iac.Authenticate(profileURL, "profile email")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// We store the authInfo in a cookie. This information will be later needed
	// to validate the callback request from the authentication server.
	err = s.storeAuthInfo(w, r, authInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Redirect to the authentication server so that the user can authorize it.
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// callbackHandler handles the callback from the authentication server.
func (s *client) callbackHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve the authentication info from the cookie.
	authInfo, err := s.getAuthInfo(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the callback using authInfo and the current request.
	code, err := s.iac.ValidateCallback(authInfo, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// We now fetch the profile of the user so we know more about the user.
	// Depending on the authentication server, this information might be more
	// or less complete. However, ".Me" must always be present.
	profile, err := s.iac.FetchProfile(authInfo, code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the ".Me" - please note that loopback (localhost) is invalid.
	if err := indieauth.IsValidProfileURL(profile.Me); err != nil {
		http.Error(w, fmt.Sprintf("invalid 'me': %s", err), http.StatusBadRequest)
		return
	}

	// The user is now logged in to your application. We simply display a simple
	// page with the profile information. However, in your application, you likely
	// want to create a session cookie (or something similar) to know that the user
	// is logged in.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = loggedInTemplate.Execute(w, profile)
}

// storeAuthInfo stores [indieauth.AuthInfo] into a cookie. This information is
// required to then validate the request once the callback is received. Note that
// this is just an example. You could use other methods, such as encoding with JWT
// tokens, a database, you name it.
func (s *client) storeAuthInfo(w http.ResponseWriter, r *http.Request, i *indieauth.AuthInfo) error {
	data, err := json.Marshal(i)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oauthCookieName,
		Value:    base64.StdEncoding.EncodeToString(data),
		Expires:  time.Now().Add(time.Minute * 10),
		Secure:   r.URL.Scheme == "https",
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})

	return nil
}

// getAuthInfo gets the [indieauth.AuthInfo] stored into a cookie.
func (s *client) getAuthInfo(w http.ResponseWriter, r *http.Request) (*indieauth.AuthInfo, error) {
	cookie, err := r.Cookie(oauthCookieName)
	if err != nil {
		return nil, err
	}

	value, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil, err
	}

	var i *indieauth.AuthInfo
	err = json.Unmarshal([]byte(value), &i)
	if err != nil {
		return nil, err
	}

	// Delete cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     oauthCookieName,
		MaxAge:   -1,
		Secure:   r.URL.Scheme == "https",
		Path:     "/",
		HttpOnly: true,
	})

	return i, nil
}
