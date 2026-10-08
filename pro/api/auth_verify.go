package api

import (
	"net/http"

	"github.com/impishMD/jeh/db"
)

func VerifySessionByEmail(session *db.Session, w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	return
}
