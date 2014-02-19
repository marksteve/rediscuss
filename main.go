package main

import (
  "encoding/base64"
  "encoding/json"
  "flag"
  "io/ioutil"
  "log"
  "net/http"
  "strings"
  "time"

  "github.com/drone/routes"
  "github.com/dustin/randbo"
  "github.com/hoisie/redis"
)

var redisAddr string
var redisKey string
var port string

var redisCli redis.Client

func key(keys ...string) string {
  keys = append([]string{redisKey}, keys...)
  return strings.Join(keys, ":")
}

type Post struct {
  Id        string `json:"id"`
  Re        string `json:"re"`
  Author    string `json:"author"`
  Timestamp int64  `json:"timestamp"`
  Content   string `json:"content"`
  Votes     int    `json:"votes"`
}

type Posts struct {
  Posts []Post `json:"posts"`
}

type Error struct {
  Message string `json:"message"`
}

func jsonError(w http.ResponseWriter, message string, code int) {
  http.Error(w, "", code)
  routes.ServeJson(w, &Error{message})
}

func getPosts(w http.ResponseWriter, r *http.Request) {
  res := r.URL.Query().Get(":res")
  postIds, err := redisCli.Smembers(key(res))
  if err != nil {
    jsonError(w, "Oops.", http.StatusInternalServerError)
    return
  }
  posts := make([]Post, 0)
  for _, id := range postIds {
    var post Post
    redisCli.Hgetall(key(res, string(id)), &post)
    posts = append(posts, post)
  }
  routes.ServeJson(w, &Posts{posts})
}

func createPost(w http.ResponseWriter, r *http.Request) {
  defer r.Body.Close()
  buf := make([]byte, 9)
  randbo.New().Read(buf)
  id := base64.URLEncoding.EncodeToString(buf)
  res := r.URL.Query().Get(":res")
  post := Post{
    Id:        id,
    Timestamp: time.Now().UnixNano(),
  }
  data, err := ioutil.ReadAll(r.Body)
  if err != nil {
    jsonError(w, "Invalid body.", http.StatusBadRequest)
    return
  }
  err = json.Unmarshal(data, &post)
  if err != nil {
    jsonError(w, "Invalid JSON.", http.StatusBadRequest)
    return
  }
  if post.Re != "" {
    if ok, _ := redisCli.Exists(key(res, post.Re)); !ok {
      jsonError(w, "Invalid re.", http.StatusBadRequest)
      return
    }
  }
  redisCli.Hmset(key(res, id), post)
  redisCli.Sadd(key(res), []byte(id))
  routes.ServeJson(w, post)
}

func checkPost(w http.ResponseWriter, r *http.Request) {
  params := r.URL.Query()
  res := params.Get(":res")
  id := params.Get(":id")
  if ok, _ := redisCli.Exists(key(res, id)); !ok {
    jsonError(w, "Missing post", http.StatusNotFound)
    return
  }
}

func updatePost(w http.ResponseWriter, r *http.Request) {
}

func delPost(w http.ResponseWriter, r *http.Request) {
}

func main() {
  redisCli.Addr = redisAddr
  mux := routes.New()
  mux.Get("/:res", getPosts)
  mux.Post("/:res", createPost)
  mux.Put("/:res/:id", updatePost)
  mux.Del("/:res/:id", delPost)
  mux.FilterParam("id", checkPost)
  log.Fatal(http.ListenAndServe(port, mux))
}

func init() {
  flag.StringVar(&redisAddr, "redisAddr", "127.0.0.1:6379", "redis address")
  flag.StringVar(&redisKey, "redisKey", "rediscuss", "redis key")
  flag.StringVar(&port, "port", ":9000", "listen to port")
  flag.Parse()
}
