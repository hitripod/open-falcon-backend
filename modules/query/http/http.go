package http

import (
	"encoding/json"
	"net/http"
	_ "net/http/pprof"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/Cepave/open-falcon-backend/modules/query/g"
	"github.com/astaxie/beego/orm"
	_ "github.com/go-sql-driver/mysql"
	"github.com/juju/errors"
)

type Dto struct {
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func InitDatabase() error {
	config := g.Config()
	// set default database
	//
	if err := orm.RegisterDataBase("default", "mysql", config.Db.Addr, config.Db.Idle, config.Db.Max); err != nil {
		return errors.Annotate(err, "init BeegoOrm for default database has error")
	}
	// register model
	orm.RegisterModel(new(Host), new(Grp), new(Grp_host), new(Grp_tpl), new(Plugin_dir), new(Tpl))

	// set grafana database
	strConn := strings.Replace(config.Db.Addr, "falcon_portal", "grafana", 1)

	if err := orm.RegisterDataBase("grafana", "mysql", strConn, config.Db.Idle, config.Db.Max); err != nil {
		return errors.Annotate(err, "init BeegoOrm for grafana database has error")
	}
	orm.RegisterModel(new(Province), new(City), new(Idc))

	if err := orm.RegisterDataBase("apollo", "mysql", config.ApolloDB.Addr, config.ApolloDB.Idle, config.ApolloDB.Max); err != nil {
		return errors.Annotate(err, "init BeegoOrm for apollo database has error")
	}
	if err := orm.RegisterDataBase("boss", "mysql", config.BossDB.Addr, config.BossDB.Idle, config.BossDB.Max); err != nil {
		return errors.Annotate(err, "init BeegoOrm for boss database has error")
	}

	orm.RegisterModel(new(Contacts), new(Hosts), new(Idcs), new(Ips), new(Platforms))
	if err := orm.RegisterDataBase("gz_nqm", "mysql", config.Nqm.Addr, config.Nqm.Idle, config.Nqm.Max); err != nil {
		return errors.Annotate(err, "init BeegoOrm for gz_nqm database has error")
	}

	orm.RegisterModel(new(Nqm_node))

	if config.Debug == true {
		orm.Debug = true
	}

	return nil
}

func Start() {
	if !g.Config().Http.Enabled {
		log.Warn("http.enabled is disabled in configuration")
		return
	}

	// config http routes
	configCommonRoutes()
	configProcHttpRoutes()
	configGraphRoutes()
	configAPIRoutes()
	configAlertRoutes()
	configGrafanaRoutes()
	configZabbixRoutes()
	configNqmRoutes()
	configNQMRoutes()

	// start mysql database
	if err := InitDatabase(); err != nil {
		log.Errorf("%s", errors.ErrorStack(err))
	}

	go SyncHostsAndContactsTable()

	// start http server
	addr := g.Config().Http.Listen
	s := &http.Server{
		Addr:           addr,
		MaxHeaderBytes: 1 << 30,
	}

	log.Println("http.Start ok, listening on", addr)
	log.Fatalln(s.ListenAndServe())
}

func RenderJson(w http.ResponseWriter, v interface{}) {
	bs, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Write(bs)
}

func RenderDataJson(w http.ResponseWriter, data interface{}) {
	RenderJson(w, Dto{Msg: "success", Data: data})
}

func RenderMsgJson(w http.ResponseWriter, msg string) {
	RenderJson(w, map[string]string{"msg": msg})
}

func AutoRender(w http.ResponseWriter, data interface{}, err error) {
	if err != nil {
		RenderMsgJson(w, err.Error())
		return
	}
	RenderDataJson(w, data)
}

func StdRender(w http.ResponseWriter, data interface{}, err error) {
	if err != nil {
		w.WriteHeader(400)
		RenderMsgJson(w, err.Error())
		return
	}
	RenderJson(w, data)
}
