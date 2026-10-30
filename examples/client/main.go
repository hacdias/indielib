package main

import (
	"errors"
	"flag"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.hacdias.com/indielib/indieauth"
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
		iac:       indieauth.NewClient(clientID, callbackURI, nil),
		authInfos: map[string]*indieauth.AuthInfo{},
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

	// shortcut: pending logins live in memory and abandoned ones are never
	// removed, use a store with expiry in production.
	authInfosMu sync.Mutex
	authInfos   map[string]*indieauth.AuthInfo
}

// indexHandler serves a simple index page with a login form.
func (c *client) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexTemplate))
}

// loginHandler handles the login process after submitting the domain via the
// index page.
func (c *client) loginHandler(w http.ResponseWriter, r *http.Request) {
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
	authInfo, redirect, err := c.iac.Authenticate(r.Context(), profileURL, "profile email")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// We store the authInfo on the server. This information will be later needed
	// to validate the callback request from the authentication server.
	c.storeAuthInfo(w, authInfo)

	// Redirect to the authentication server so that the user can authorize it.
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// callbackHandler handles the callback from the authentication server.
func (c *client) callbackHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve the authentication info from the cookie.
	authInfo, err := c.getAuthInfo(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the callback using authInfo and the current request.
	code, err := c.iac.ValidateCallback(authInfo, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// We now fetch the profile of the user so we know more about the user.
	// Depending on the authentication server, this information might be more
	// or less complete. However, ".Me" must always be present.
	profile, err := c.iac.FetchProfile(r.Context(), authInfo, code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// The user is now logged in to your application. We simply display a simple
	// page with the profile information. However, in your application, you likely
	// want to create a session cookie (or something similar) to know that the user
	// is logged in.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = loggedInTemplate.Execute(w, profile)
}

// storeAuthInfo stores [indieauth.AuthInfo] on the server, keyed by its state,
// and puts the state in a cookie. This information is required to then validate
// the request once the callback is received. The callback checks trust it, so it
// must not be stored where the user can change it, such as in a cookie.
func (c *client) storeAuthInfo(w http.ResponseWriter, i *indieauth.AuthInfo) {
	c.authInfosMu.Lock()
	c.authInfos[i.State] = i
	c.authInfosMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     oauthCookieName,
		Value:    i.State,
		Expires:  time.Now().Add(time.Minute * 10),
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})
}

// getAuthInfo gets the [indieauth.AuthInfo] for the state stored in the cookie.
func (c *client) getAuthInfo(w http.ResponseWriter, r *http.Request) (*indieauth.AuthInfo, error) {
	cookie, err := r.Cookie(oauthCookieName)
	if err != nil {
		return nil, err
	}

	c.authInfosMu.Lock()
	i, ok := c.authInfos[cookie.Value]
	delete(c.authInfos, cookie.Value)
	c.authInfosMu.Unlock()

	if !ok {
		return nil, errors.New("unknown login, please try again")
	}

	// Delete cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     oauthCookieName,
		MaxAge:   -1,
		Path:     "/",
		HttpOnly: true,
	})

	return i, nil
}
