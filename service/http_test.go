package service

import (
	"encoding/hex"
	"fmt"
	"github.com/stretchr/testify/assert"
	"gopkg.in/jcmturner/gokrb5.v1/client"
	"gopkg.in/jcmturner/gokrb5.v1/credentials"
	"gopkg.in/jcmturner/gokrb5.v1/iana/nametype"
	"gopkg.in/jcmturner/gokrb5.v1/keytab"
	"gopkg.in/jcmturner/gokrb5.v1/messages"
	"gopkg.in/jcmturner/gokrb5.v1/testdata"
	"gopkg.in/jcmturner/gokrb5.v1/types"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestService_SPNEGOKRB_NoAuthHeader(t *testing.T) {
	s := httpServer()
	defer s.Close()
	r, _ := http.NewRequest("GET", s.URL, nil)
	httpResp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("Request error: %v\n", err)
	}
	assert.Equal(t, http.StatusUnauthorized, httpResp.StatusCode, "Status code in response to client with no SPNEGO not as expected")
	assert.Equal(t, "Negotiate", httpResp.Header.Get("WWW-Authenticate"), "Negitation header not set by server.")
}

func TestService_SPNEGOKRB_ValidUser(t *testing.T) {
	s := httpServer()
	defer s.Close()

	cl := getClient()
	sname := types.PrincipalName{
		NameType:   nametype.KRB_NT_PRINCIPAL,
		NameString: []string{"HTTP", "host.test.gokrb5"},
	}
	b, _ := hex.DecodeString(testdata.HTTP_KEYTAB)
	kt, _ := keytab.Parse(b)
	st := time.Now().UTC()
	tkt, sessionKey, err := messages.NewTicket(cl.Credentials.CName, cl.Credentials.Realm,
		sname, "TEST.GOKRB5",
		types.NewKrbFlags(),
		kt,
		18,
		1,
		st,
		st,
		st.Add(time.Duration(24)*time.Hour),
		st.Add(time.Duration(48)*time.Hour),
	)
	if err != nil {
		t.Fatalf("Error getting test ticket: %v", err)
	}

	r, _ := http.NewRequest("GET", s.URL, nil)
	err = client.SetSPNEGOHeader(*cl.Credentials, tkt, sessionKey, r)
	if err != nil {
		t.Fatalf("Error setting client SPNEGO header: %v", err)
	}
	httpResp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("Request error: %v\n", err)
	}
	assert.Equal(t, http.StatusOK, httpResp.StatusCode, "Status code in response to client SPNEGO request not as expected")
}

func TestService_SPNEGOKRB_Replay(t *testing.T) {
	s := httpServer()
	defer s.Close()

	cl := getClient()
	sname := types.PrincipalName{
		NameType:   nametype.KRB_NT_PRINCIPAL,
		NameString: []string{"HTTP", "host.test.gokrb5"},
	}
	b, _ := hex.DecodeString(testdata.HTTP_KEYTAB)
	kt, _ := keytab.Parse(b)
	st := time.Now().UTC()
	tkt, sessionKey, err := messages.NewTicket(cl.Credentials.CName, cl.Credentials.Realm,
		sname, "TEST.GOKRB5",
		types.NewKrbFlags(),
		kt,
		18,
		1,
		st,
		st,
		st.Add(time.Duration(24)*time.Hour),
		st.Add(time.Duration(48)*time.Hour),
	)
	if err != nil {
		t.Fatalf("Error getting test ticket: %v", err)
	}

	r, _ := http.NewRequest("GET", s.URL, nil)
	err = client.SetSPNEGOHeader(*cl.Credentials, tkt, sessionKey, r)
	if err != nil {
		t.Fatalf("Error setting client SPNEGO header: %v", err)
	}

	httpResp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("Request error: %v\n", err)
	}
	assert.Equal(t, http.StatusOK, httpResp.StatusCode, "Status code in response to client SPNEGO request not as expected")

	// Do not Set the SPNEGO header again so this should try to replay the tokens
	httpResp, err = http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("Request error: %v\n", err)
	}
	assert.Equal(t, http.StatusUnauthorized, httpResp.StatusCode, "Status code in response to client with no SPNEGO not as expected. Expected a replay to be detected.")
}

func httpServer() *httptest.Server {
	l := log.New(ioutil.Discard, "GOKRB5 Service Tests: ", log.Ldate|log.Ltime|log.Lshortfile)
	b, _ := hex.DecodeString(testdata.HTTP_KEYTAB)
	kt, _ := keytab.Parse(b)
	th := http.HandlerFunc(testAppHandler)
	s := httptest.NewServer(SPNEGOKRB5Authenticate(th, kt, "", l))
	return s
}

func testAppHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	ctx := r.Context()
	fmt.Fprintf(w, "<html>\nTEST.GOKRB5 Handler\nAuthenticed user: %s\nUser's realm: %s\n</html>", ctx.Value(CTXKeyCredentials).(credentials.Credentials).Username, ctx.Value(CTXKeyCredentials).(credentials.Credentials).Realm)
	return
}
