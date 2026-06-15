package metrics

func Init() {
	http_init()
	sso_init()
	db_init()
}
