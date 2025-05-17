package main

// Импортируем пакеты
import (
	"backend/database"
	"backend/handlers"
	"log"
	"net/http"

	"github.com/rs/cors"
)

// Главная функция
func main() {
	env := database.ParseEnv()
	mux := http.NewServeMux()
	mux.HandleFunc("/putStatus", handlers.PutStatus)
	mux.HandleFunc("/ContainerList", handlers.ContainerList)
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	})

	handler := c.Handler(mux)
	if err := http.ListenAndServe(":"+env.Port, handler); err != nil {
		log.Fatal(err)
	}
}
