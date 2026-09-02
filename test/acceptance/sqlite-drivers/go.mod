module github.com/kefyusuf/dbprobe/test/acceptance/sqlite-drivers

go 1.25.0

require (
	github.com/kefyusuf/dbprobe v0.0.0
	github.com/ncruces/go-sqlite3 v0.35.3
	modernc.org/sqlite v1.57.0
)

replace github.com/kefyusuf/dbprobe => ../../..
