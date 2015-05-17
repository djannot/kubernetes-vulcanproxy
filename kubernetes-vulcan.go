package main

import (
  "fmt"
  "bytes"
  "net/http"
  "net/url"
  "github.com/bitly/go-simplejson"
  "strconv"
  "strings"
  "flag"
  )

type Response struct {
  Code int
  Body string
  Headers http.Header
}

type kubernetesService struct {
  Id string
}

var kubernetesEndPoint string
var etcdEndPoint string
var etcdRootKey string

func main() {
  kubernetesEndPointPtr := flag.String("KubernetesEndPoint", "", "The Kubernetes API Endpoint")
  etcdEndPointPtr := flag.String("EtcdEndPoint", "", "The Etcd API Endpoint")
  etcdRootKeyPtr := flag.String("EtcdRootKey", "", "The Etcd root key to use for kubernetes-vulcan")
  flag.Parse()
  kubernetesEndPoint = *kubernetesEndPointPtr
  etcdEndPoint = *etcdEndPointPtr
  etcdRootKey = *etcdRootKeyPtr

  kubernetesServices := make(map[string]kubernetesService)
  // Get all the kubernetes Services from the ETCD API
  etcdVulcanResponse, err := httpRequest(etcdEndPoint, "GET", "/keys" + etcdRootKey + "/", nil, "")
  if err != nil {
    fmt.Println(err)
  }
  if etcdVulcanResponse.Code == 200 {
    jsonServices, _ := simplejson.NewJson([]byte(etcdVulcanResponse.Body))
    services, _ := jsonServices.Get("node").Get("nodes").Array()
    // For each kubernetes Service
    for service, _ := range services {
      key, _ := jsonServices.Get("node").Get("nodes").GetIndex(service).Get("key").String()
      serviceId := key[len(etcdRootKey) + 1:]
      // Get the value of the ETCD backend key corresponding to this kubernetes Service
      etcdGetBackendResponse, err := httpRequest(etcdEndPoint, "GET", "/keys" + etcdRootKey + "/" + serviceId + "/backend", nil , "")
      if err != nil {
        fmt.Println(err)
      }
      // If the ETCD backend key corresponding to this kubernetes Service exists
      if etcdGetBackendResponse.Code == 200 {
        jsonBackend, _ := simplejson.NewJson([]byte(etcdGetBackendResponse.Body))
        backend, _ := jsonBackend.Get("node").Get("value").String()
        // Get the value of the ETCD vulcan server keys corresponding to this kubernetes Service
        path := "/keys/vulcand/backends/" + backend + "/servers/"
        etcdGetServersResponse, err := httpRequest(etcdEndPoint, "GET", path , nil , "")
        if err != nil {
          fmt.Println(err)
        }
        // If there's at least one ETCD server key
        if etcdGetServersResponse.Code == 200 {
          jsonServers, _ := simplejson.NewJson([]byte(etcdGetServersResponse.Body))
          if _, ok := jsonServers.Get("node").CheckGet("nodes"); ok {
            key, _ := jsonServers.Get("node").Get("nodes").GetIndex(0).Get("key").String()
            directory := key[len(path) - 5:]
            ip := directory[:strings.LastIndex(directory, "-")]
            port := directory[strings.LastIndex(directory, "-") + 1:]
            // Check the service is still running
            kubernetesServiceResponse, err := httpRequest(kubernetesEndPoint, "GET", "/api/v1beta1/services/" + serviceId, nil , "")
            if err != nil {
              fmt.Println(err)
            }
            jsonService, _ := simplejson.NewJson([]byte(kubernetesServiceResponse.Body))
            portalIp, _ := jsonService.Get("portalIP").String()
            servicePort, _ := jsonService.Get("port").Int()
            // If the service is still running and the IP and ports haven't changed
            if kubernetesServiceResponse.Code == 200 && ip == portalIp && port == strconv.Itoa(servicePort) {
              fmt.Println("Kubernetes Service " + serviceId + ": The service which associated with " + ip + " and the port " + port + " is still running")
              kubernetesService := kubernetesService{Id: serviceId}
              kubernetesServices[serviceId] = kubernetesService
            } else {
              key := "/keys/vulcand/backends/" + backend + "/servers/" + ip + "-" + port
              etcdDeleteServerResponse, err := httpRequest(etcdEndPoint, "DELETE", key, nil , "")
              if err != nil {
                fmt.Println(err)
              }
              if etcdDeleteServerResponse.Code == 307 {
                etcdDeleteServerResponse, _ = httpRequest(etcdDeleteServerResponse.Headers["Location"][0], "DELETE", "", nil, "")
                if err != nil {
                  fmt.Println(err)
                }
              }
              if etcdDeleteServerResponse.Code == 200 {
                fmt.Println("Kubernetes Service " + serviceId + ": The service which associated with " + ip + " and the port " + port + " isn't running anymore and the corresponding vulcan server has been deleted")
              } else {
                fmt.Println("Kubernetes Service " + serviceId + ": The service which associated with " + ip + " and the port " + port + " isn't running anymore, but the corresponding vulcan server hasn't been deleted")
              }
            }
          }
        }
      }  else {
        fmt.Println("ETCD frontend and backend keys corresponding to the kubernetes Service with the ID " + serviceId + " aren't configured properly. Can't check the Vulcan servers for this application")
      }
    }
  }

  // Get all the kubernetes Services from the kubernetes API
  kubernetesServicesResponse, err := httpRequest(kubernetesEndPoint, "GET", "/api/v1beta1/services", nil , "")
  if err != nil {
    fmt.Println(err)
  }
  jsonServices, _ := simplejson.NewJson([]byte(kubernetesServicesResponse.Body))
  services, _ := jsonServices.Get("items").Array()
  // For each kubernetes Service
  for service, _ := range services {
    serviceId, _ := jsonServices.Get("items").GetIndex(service).Get("id").String()
    portalIp, _ := jsonServices.Get("items").GetIndex(service).Get("portalIP").String()
    port, _ := jsonServices.Get("items").GetIndex(service).Get("port").Int()

    // Get the value of the ETCD frontend and backend keys corresponding to this kubernetes Service
    etcdGetFrontendResponse, err := httpRequest(etcdEndPoint, "GET", "/keys" + etcdRootKey + "/" + serviceId + "/frontend", nil , "")
    if err != nil {
      fmt.Println(err)
    }
    etcdGetBackendResponse, err := httpRequest(etcdEndPoint, "GET", "/keys" + etcdRootKey + "/" + serviceId + "/backend", nil , "")
    if err != nil {
      fmt.Println(err)
    }
    // If the ETCD frontend and backend keys corresponding to this kubernetes Service exist
    if etcdGetFrontendResponse.Code == 200 && etcdGetBackendResponse.Code == 200 {
      jsonFrontend, _ := simplejson.NewJson([]byte(etcdGetFrontendResponse.Body))
      jsonBackend, _ := simplejson.NewJson([]byte(etcdGetBackendResponse.Body))
      frontend, _ := jsonFrontend.Get("node").Get("value").String()
      backend, _ := jsonBackend.Get("node").Get("value").String()
      // Create the vulcan proxy server entry corresponding to this IP and port in ETCD if it's not already defined
      found := false
      for _, kubernetesService := range kubernetesServices {
        if kubernetesService.Id == serviceId {
          found = true
        }
      }
      if found == false {
        key := "/keys/vulcand/backends/" + backend + "/servers/" + portalIp + "-" + strconv.Itoa(port)
        value := `value={"Id":"` + portalIp + "-" + strconv.Itoa(port) + `","URL":"http://` + portalIp + `:` + strconv.Itoa(port) + `"}`
        headers := make(map[string][]string)
        headers["Content-Type"] = []string{"application/x-www-form-urlencoded"}
        etcdVulcanResponse, _ := httpRequest(etcdEndPoint, "PUT", key, headers, value)
        if err != nil {
          fmt.Println(err)
        }
        if etcdVulcanResponse.Code == 307 {
          etcdVulcanResponse, _ = httpRequest(etcdVulcanResponse.Headers["Location"][0], "PUT", "", headers, value)
          if err != nil {
            fmt.Println(err)
          }
        }
        if etcdVulcanResponse.Code == 200 || etcdVulcanResponse.Code == 201 {
          fmt.Println("Kubernetes Service " + serviceId + ": Key " + key + " created or updated in ETCD with " + value + " for the frontend " + frontend)
        } else {
          fmt.Println("Kubernetes Service " + serviceId + ": Cannot create the key " + key + " in ETCD with " + value + " for the frontend " + frontend)
        }
      }
    } else {
      fmt.Println("ETCD frontend and backend keys corresponding to the kubernetes Service with the ID " + serviceId + " aren't configured properly. Can't configure the Vulcan servers for this service")
    }
  }
}

func httpRequest(endPoint string, method string, path string, headers map[string][]string, bodyString string) (Response, error) {
  fullUrl := endPoint + path
  v := url.Values{}
  if len(headers) > 0 {
    v = headers
    fullUrl += "?" + v.Encode()
  }
  httpClient := &http.Client{}
  req, err := http.NewRequest(method, fullUrl, strings.NewReader(bodyString))
  if err != nil {
    return Response{}, err
  }
  req.Header = headers
  resp, err := httpClient.Do(req)
  if err != nil {
    return Response{}, err
  }
  buf := new(bytes.Buffer)
  buf.ReadFrom(resp.Body)
  body := buf.String()
  response := Response{
    Code: resp.StatusCode,
    Body: body,
    Headers: resp.Header,
  }
  return response, nil
}
