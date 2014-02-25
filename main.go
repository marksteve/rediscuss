package main

import (
  "encoding/base64"
  "encoding/json"
  "flag"
  "io/ioutil"
  "log"
  "net/http"
  "os"
  "strings"
  "time"

  "github.com/dustin/randbo"
  "github.com/gorilla/context"
  "github.com/hoisie/redis"
  "github.com/marksteve/routes"
  "github.com/memcachier/bcrypt"
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

type Creds struct {
  Name     string `json:"name"`
  Password string `json:"password"`
}

type User struct {
  Name  string `json:"name"`
  Token string `json:"token"`
  Hash  string `json:"-"`
}

func jsonError(w http.ResponseWriter, message string, code int) {
  routes.ServeJson(w, &Error{message}, code)
}

func getJson(v interface{}, w http.ResponseWriter, r *http.Request) {
  data, err := ioutil.ReadAll(r.Body)
  if err != nil {
    jsonError(w, "Invalid body.", http.StatusBadRequest)
    return
  }
  err = json.Unmarshal(data, &v)
  if err != nil {
    jsonError(w, "Invalid JSON.", http.StatusBadRequest)
    return
  }
}

func genRandString(n int) string {
  buf := make([]byte, n)
  randbo.New().Read(buf)
  id := base64.URLEncoding.EncodeToString(buf)
  return id
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
  routes.ServeJson(w, &Posts{posts}, http.StatusOK)
}

func createPost(w http.ResponseWriter, r *http.Request) {
  withUser(w, r)
  author, ok := context.GetOk(r, "user")
  if !ok {
    jsonError(w, "You need to login to post.", http.StatusUnauthorized)
    return
  }
  defer r.Body.Close()
  id := genRandString(9)
  res := r.URL.Query().Get(":res")
  post := Post{}
  getJson(&post, w, r)
  post.Id = id
  if post.Re != "" {
    if ok, _ := redisCli.Exists(key(res, post.Re)); !ok {
      jsonError(w, "Invalid re.", http.StatusBadRequest)
      return
    }
  }
  post.Author = author.(User).Name
  post.Timestamp = time.Now().UnixNano()
  post.Votes = 0
  redisCli.Hmset(key(res, id), post)
  redisCli.Sadd(key(res), []byte(id))
  routes.ServeJson(w, post, http.StatusOK)
}

func withPost(w http.ResponseWriter, r *http.Request) {
  params := r.URL.Query()
  res := params.Get(":res")
  id := params.Get(":id")
  var p Post
  err := redisCli.Hgetall(key(res, id), &p)
  if err != nil {
    jsonError(w, "Missing post", http.StatusNotFound)
    return
  }
  context.Set(r, "post", p)
}

func withUser(w http.ResponseWriter, r *http.Request) {
  auth := r.Header.Get("Authorization")
  if auth == "" {
    return
  }
  authSplit := strings.SplitN(auth, " ", 2)
  if authSplit[0] != "token" {
    return
  }
  token := authSplit[1]
  name, err := redisCli.Get(key("tokens", token))
  if len(name) < 1 || err != nil {
    return
  }
  // FIXME: Cache
  var u User
  err = redisCli.Hgetall(key("users", string(name)), &u)
  if err != nil {
    return
  }
  context.Set(r, "user", u)
}

func setToken(name string) (string, error) {
  token := genRandString(27)
  err := redisCli.Setex(key("tokens", token), 60*60*24*7, []byte(name))
  if err != nil {
    return "", err
  }
  return token, nil
}

func updatePost(w http.ResponseWriter, r *http.Request) {
  context.Clear(r)
}

func delPost(w http.ResponseWriter, r *http.Request) {
  context.Clear(r)
}

func register(w http.ResponseWriter, r *http.Request) {
  var c Creds
  getJson(&c, w, r)
  if ok, _ := redisCli.Exists(key("users", c.Name)); ok {
    jsonError(w, "Invalid name.", http.StatusBadRequest)
    return
  }
  salt, err := bcrypt.GenSalt(10)
  if err != nil {
    jsonError(w, "Oops.", http.StatusInternalServerError)
    return
  }
  hash, err := bcrypt.Crypt(c.Password, salt)
  if err != nil {
    jsonError(w, "Oops.", http.StatusInternalServerError)
    return
  }
  u := User{
    Name: c.Name,
    Hash: hash,
  }
  u.Token, err = setToken(c.Name)
  if err != nil {
    jsonError(w, "Oops.", http.StatusInternalServerError)
    return
  }
  err = redisCli.Hmset(key("users", c.Name), u)
  if err != nil {
    jsonError(w, "Oops.", http.StatusInternalServerError)
    return
  }
  routes.ServeJson(w, u, http.StatusOK)
}

func login(w http.ResponseWriter, r *http.Request) {
  var c Creds
  getJson(&c, w, r)
  var u User
  err := redisCli.Hgetall(key("users", c.Name), &u)
  if err == nil {
    jsonError(w, "Invalid login.", http.StatusUnauthorized)
    return
  }
  if ok, err := bcrypt.Verify(c.Password, u.Hash); !ok || err != nil {
    jsonError(w, "Invalid login.", http.StatusUnauthorized)
    return
  }
  u.Token, err = setToken(c.Name)
  if err != nil {
    jsonError(w, "Oops.", http.StatusInternalServerError)
    return
  }
  routes.ServeJson(w, u, http.StatusOK)
}

func main() {
  redisCli.Addr = redisAddr
  mux := routes.New()
  mux.Post("/register", register)
  mux.Post("/login", login)
  mux.Get("/:res", getPosts)
  mux.Post("/:res", createPost)
  mux.Put("/:res/:id", updatePost)
  mux.Del("/:res/:id", delPost)
  mux.FilterParam("id", withPost)
  mux.FilterParam("id", withUser)
  wd, _ := os.Getwd()
  mux.Static("/s/", wd)
  log.Fatal(http.ListenAndServe(port, mux))
}

func init() {
  flag.StringVar(&redisAddr, "redisAddr", "127.0.0.1:6379", "redis address")
  flag.StringVar(&redisKey, "redisKey", "rediscuss", "redis key")
  flag.StringVar(&port, "port", ":9000", "listen to port")
  flag.Parse()
}
