package handlers

// Импортируем пакеты
import (
	"backend/database"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Структура для отправки данных на фронтенд
type ToFront struct {
	IP        string    `json:"ip"`
	Status    string    `json:"status"`
	CPU       string    `json:"cpu"`
	Memory    string    `json:"memory"`
	Timestamp time.Time `json:"timestamp"`
	Datestamp time.Time `json:"datestamp"`
}

// Получение списка контейнеров из БД и отправляет информацию на фронтенд
func ContainerList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Println("Error: wrong http method")
		return
	}
	reqs := []ToFront{}
	conts := []database.DBContainer{}
	db, err := database.DbConnect()
	if err != nil {
		log.Println(err)
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Println(err)
		return
	}

	defer func(sqlDB *sql.DB) {
		err := sqlDB.Close()
		if err != nil {
			log.Println(err)
		}
	}(sqlDB)

	query := db.Order("timestamp ASC").Find(&conts)
	if query.Error != nil {
		log.Println(query.Error)
		return
	}

	for i := range conts {
		reqs = append(reqs, ToFront{
			IP:        conts[i].IP,
			Status:    conts[i].Status,
			CPU:       fmt.Sprintf("%f", conts[i].CPU),
			Memory:    fmt.Sprintf("%d", conts[i].Memory),
			Timestamp: conts[i].Timestamp,
			Datestamp: conts[i].Datestamp,
		})
	}

	b, err := json.Marshal(reqs)
	if err != nil {
		log.Println(err)
		return
	}

	w.Header().Set("Access-Control-Allow-Methods", "GET")
	if _, err := w.Write(b); err != nil {
		log.Println(err)
	}
	w.WriteHeader(http.StatusOK)
}

// Принимает Post запрос от пингера с информацией о контейнерах и кладет в БД
func PutStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Println("Wrong method!")
		return
	}

	byteReq, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		return
	}
	reqs := []database.Request{}
	err = json.Unmarshal(byteReq, &reqs)
	if err != nil {
		log.Println(err)
		return
	}

	db, err := database.DbConnect()
	if err != nil {
		log.Println(err)
		http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Println(err)
		return
	}
	defer func(sqlDB *sql.DB) {
		err := sqlDB.Close()
		if err != nil {
			log.Println(err)
		}
	}(sqlDB)
	err = database.SaveContainer(db, reqs)
	if err != nil {
		log.Println(err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
