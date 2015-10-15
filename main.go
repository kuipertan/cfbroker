package main

import (
	"encoding/json"
	"os"
	"time"
	"fmt"
	"io/ioutil"
	"strings"
	"database/sql"
	"net/http"
	
	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/go-martini/martini"
	"github.com/kuipertan/types-cf"
	_ "github.com/go-sql-driver/mysql"
)

import log "github.com/kuipertan/log4go"


const (
	serviceID = "288b2bd6-d993-4e59-8efb-1ce87c0f5588"
	imageURL = ""
	cloudurl = "http://127.0.0.1:8765/"
)

type asyncOpRet struct {
	Response *http.Response
	Err error
}


var serviceName, tags, serviceDescription string
var dbhost, dbport, dbuser,dbpass,dbname string
var asyncChan chan asyncOpRet

var gInstanceID, gPlanID, gLastOp string


func init() {
	log.AddFilter("stdout", log.INFO, log.NewConsoleLogWriter())
	log.Info("cfbroker instance starting...")
	asyncChan = make(chan asyncOpRet, 1)
}

func asyncOp(op string, args ...interface{}) {

	var resp *http.Response
	var err error
	if op == "Provision" {
		resp, err = http.Get(cloudurl + "restapply?"+ "product_id="+ args[0].(string) + "&need_memory=" + args[1].(string))
	} else if op == "Update" {
		resp, err = http.Get(cloudurl + "restalter?"+ "product_id="+ args[0].(string) + "&need_memory=" + args[1].(string))
	} 

	asyncChan <- asyncOpRet{resp, err}
}


func getInstanceParams(guid string) (string, error) {
	db, err := sql.Open("mysql", dbuser+":"+dbpass+"@("+dbhost+":"+dbport+")/"+dbname)
	if err != nil {
		return "",err
	}

	defer db.Close()
	var params string
	err = db.QueryRow("SELECT parameters FROM ServiceInstance WHERE id=?", guid).Scan(&params)
	if err != nil {
		return "",err
	}
	return params,nil
}


type serviceInstanceResponse struct {
	DashboardURL string `json:"dashboard_url"`
}

type BindingResponse struct {
	Credentials    map[string]interface{} `json:"credentials"`
	//SyslogDrainURL string                 `json:"syslog_drain_url"`
}


func catalog() (int, []byte) {
	tagArray := []string{}
	if len(tags) > 0 {
		tagArray = strings.Split(tags, ",")
	}
	catalog := cf.Catalog{
		Services: []*cf.Service{
			{
				ID:          serviceID,
				Name:        serviceName,
				Description: serviceDescription,
				Bindable:    true,
				Plan_updateable: true,
				Tags:        tagArray,
				Metadata: &cf.ServiceMeta{
					DisplayName: serviceName,
					//ImageURL:    imageURL,
				},
				Plans: myplans, 
			},
		},
	}
	json, err := json.Marshal(catalog)
	if err != nil {
		log.Error("Um, how did we fail to marshal this catalog:")
		return 500, []byte("{}")
	}
	return 200, json
}

func incomplete(req *http.Request) bool {
	ps := req.URL.Query()
	
	for k,v := range ps {
		if strings.ToLower(k) == "accepts_incomplete" { 
			if strings.ToLower(v[0]) == "true" { 
    			return true
			}
		}
	}
	return false
}

func unprocessable()(int, []byte){
	return 422, []byte(`{ "error": "AsyncRequired", "description": "This service plan requires client support for asynchronous service operations." }`)
} 


func serviceInstance(params martini.Params, req *http.Request, op string) (int, []byte) {
	if !incomplete(req){
		return unprocessable()
	}

	instanceID := params["instance_id"]

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return 500, []byte(`{"error":"` + err.Error() + `"}`)
	}	

	//Var req_param map[string]interface{}
	var v interface{}
	//err = json.Unmarshal(body, &req_param)	
	err = json.Unmarshal(body, &v)	
	if err != nil {
		return 500, []byte(`{"error":"` + err.Error() + `"}`)
	}
	var req_param = v.(map[string]interface{})

	var planID string
	for _, p := range myplans{
		if p.ID == req_param["plan_id"] {
			planID = p.Name
			break
		}
	}


	gLastOp = op	
	gInstanceID = instanceID
	gPlanID = planID
	go asyncOp(op, instanceID, planID[1:])

	return 202, []byte("{}")
}

func provisioning(params martini.Params, req *http.Request) (int, []byte) {
	return serviceInstance(params, req, "Provision")
}

func update(params martini.Params, req *http.Request) (int, []byte) {
	return serviceInstance(params, req, "Update")
}

func unprovisioning(params martini.Params) (int, []byte){
	instanceID := params["instance_id"]
	db, err := sql.Open("mysql", dbuser+":"+dbpass+"@("+dbhost+":"+dbport+")/"+dbname)
	if err != nil {
		return 500, []byte(`{"state": "failed", "description":"` + err.Error() + `"}`)
       	}
			
	defer db.Close()
	query := `UPDATE ServiceInstance SET provisioned=0 where id='`+instanceID+`'` 
	_, err = db.Exec(query)
	if err !=nil {
		return 500, []byte(`{"state": "failed", "description":"` + err.Error() + `"}`)
       	}
/*
	gLastOp = op	
	gInstanceID = instanceID
	gPlanID = planID
        go asyncOp(op, instanceID, planID[1:])
*/
	return 200,[]byte("{}")	
}


func lastOperation(params martini.Params) (int, []byte) {
	instanceID := params["instance_id"]
	if instanceID != gInstanceID {
		return 200, []byte(`{"state:"failed", "description":"Brokdr civil error, instance id is deranged"}`)
	}

	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(1e9)
		timeout <- true
	} ()

	select {
		case ret := <-  asyncChan:
			if ret.Err != nil {
				return 200, []byte(`{"state": "failed", "description": "` +  ret.Err.Error() +  `"}`)
			}


			if ret.Response.StatusCode != 200 {
				return 200, []byte(`{"state": "failed", "description": " The backend returns ` + ret.Response.Status + `"}`)
			}
			body, _ := ioutil.ReadAll(ret.Response.Body)

			if gLastOp == "Provision" {
				db, err := sql.Open("mysql", dbuser+":"+dbpass+"@("+dbhost+":"+dbport+")/"+dbname)
				if err != nil {
					return 200, []byte(`{"state": "failed", "description":"` + err.Error() + `"}`)
          			}
				
				defer db.Close()
				query := `INSERT INTO ServiceInstance(id,plan_id,parameters,provisioned) VALUES('` + 
					gInstanceID + `','` + gPlanID + `', '` + string(body) + `', 1)`
				_, err = db.Exec(query)
				if err !=nil {
					return 200, []byte(`{"state": "failed", "description":"` + err.Error() + `"}`)
          			}
			}

                	return 200, []byte(`{"state": "succeeded", "description":"Success!"}`)

		case <- timeout:
			return 200, []byte(`{"state": "in progress"}`)
	}
}


func binding(params martini.Params) (int, []byte) {
	instanceID := params["instance_id"]
	//serviceBindingID := params["binding_id"]

	result, err := getInstanceParams(instanceID) 
	if err != nil {
		return 500, []byte(`{"error":"` + err.Error() + `"}`)
	}

	var out map[string]interface{}
	err = json.Unmarshal([]byte(result), &out)
	if err != nil {
		return 500, []byte(`{"error":"Wrong format string from db or codis"}`)	
	}		
		
	response := BindingResponse{Credentials:make(map[string]interface{})}
	response.Credentials["product_id"] = out["product_id"]
	response.Credentials["need_memory"] = out["need_memory"]
	response.Credentials["proxypath"] = out["proxypath"]
	response.Credentials["zkaddr"] = out["zkaddr"]

	json, err := json.Marshal(response)
	if err != nil {
		return 500, []byte("{}")
	}
	return 201, json
}

func unBinding(params martini.Params) (int, []byte) {
	return 200, []byte("{}")
}

/*
func showServiceInstanceDashboard(params martini.Params) (int, string) {
	fmt.Printf("Show dashboard for service %s plan %s\n", serviceName, servicePlan)
	return 200, "Dashboard"
}
*/

func main() {
	m := martini.Classic()

	serviceName = os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "cfservice" // replace with cfenv.AppName
	}
	serviceDescription = os.Getenv("SERVICE_DESCRIPTION")
	if serviceDescription == "" {
		serviceDescription = "Service description" // replace with cfenv.AppName
	}
	tags = os.Getenv("TAGS")

	//imageURL = os.Getenv("IMAGE_URL")


	port := os.Getenv("PORT")


	appEnv, err := cfenv.Current()
	if err != nil {
		fmt.Println("cfenv out")
		return 
	} 

	sv,err := appEnv.Services.WithName("mysql")
	if err != nil {
		fmt.Println("search mysql out")
		return
	}

	dbhost = sv.Credentials["host"].(string)
	dbport=sv.Credentials["port"].(string)
	dbname="broker"
	dbuser=sv.Credentials["user"].(string)
	dbpass=sv.Credentials["password"].(string)
/*
	dbhost="127.0.0.1"
	dbport="3306"
	dbname="broker"
	dbuser="rdb"
	dbpass="rdb"
*/
	m.Get("/v2/catalog", catalog)
	m.Get("/v2/service_instances/:instance_id/last_operation", lastOperation)
	m.Put("/v2/service_instances/:instance_id", provisioning)
	m.Patch("/v2/service_instances/:instance_id", update)
	m.Delete("/v2/service_instances/:instance_id", unprovisioning)
	m.Put("/v2/service_instances/:instance_id/service_bindings/:binding_id", binding)
	m.Delete("/v2/service_instances/:instance_id/service_bindings/:binding_id", unBinding)

	// Service Instance Dashboard
	//m.Get("/dashboard", showServiceInstanceDashboard)

	m.RunOnAddr(":"+port)
}
