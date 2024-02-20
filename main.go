package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

type Cliente struct {
	Id     int `json:"id "`
	Limite int `json:"limite"`
	Saldo  int `json:"saldo"`
}

type Teste struct {
	Id   int    `json: "id"`
	Name string `json: "name"`
}

type Transaction struct {
	Valor     int    `json: "id"`
	Tipo      string `json: "tipo"` //c = crédito d = débito
	Descricao string `json: "descricao"`
}

type Response struct {
	Limite int `json: "int"`
	Saldo  int `json: "int"`
}

type Transacao struct {
	Valor       int       `json:"valor"`
	Tipo        string    `json:"tipo"`
	Descricao   string    `json:"descricao"`
	RealizadaEm time.Time `json:"realizada_em"`
}

type Saldo struct {
	Total       int       `json:"total"`
	DataExtrato time.Time `json:"data_extrato"`
	Limite      int       `json:"limite"`
}

type Extrato struct {
	Saldo             Saldo       `json: "saldo"`
	UltimasTransacoes []Transacao `json: ultimas_transacoes`
}

func main() {
	// db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	db, err := sql.Open("postgres", "host=localhost user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS cliente(id int, limite int, saldo int);")

	if err != nil {
		log.Fatal(err)
	}

	router := mux.NewRouter()

	router.HandleFunc("/clientes/{id}/transacoes", TransactionsHandler(db)).Methods("POST")
	router.HandleFunc("/clientes/{id}/extrato", ExtratosHandler(db)).Methods("GET")

	log.Fatal(http.ListenAndServe(":8000", jsonContentTypeMiddleware(router)))
}

func jsonContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(w, r)
	})
}

func ExtratosHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(mux.Vars(r)["id"])
		if err != nil {
			log.Fatal(err)
		}

		rows, err := db.Query(`select cl.saldo as total, cl.limite, ut.valor, ut.tipo, ut.descricao, ut.realizada_em from cliente cl inner join ultimas_transacoes ut on cl.id =$1`, id)

		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()

		if !rows.Next() { //tratando caso de não encontrar o usuário
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var qq Extrato
		for rows.Next() {

			var ut Transacao
			var sd Saldo
			err := rows.Scan(&sd.Total, &sd.Limite, &ut.Valor, &ut.Tipo, &ut.Descricao, &ut.RealizadaEm)
			if err != nil {
				log.Fatal(err)
			}
			qq.Saldo = sd
			qq.Saldo.DataExtrato = time.Now().UTC()
			qq.UltimasTransacoes = append(qq.UltimasTransacoes, ut)
		}

		json.NewEncoder(w).Encode(qq)

	}

}

func TransactionsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		id, err := strconv.Atoi(mux.Vars(r)["id"])
		if err != nil {
			log.Fatal(err)
		}

		rows, err := db.Query("select * from cliente where id = $1", id)

		if err != nil {
			log.Fatal(err)
		}

		columns, err := rows.Columns()
		if err != nil {
			log.Fatal(err)
		}
		if len(columns) == 0 {

			w.WriteHeader(http.StatusNotFound)
		}

		defer rows.Close()
		var cliente Cliente
		for rows.Next() {
			err := rows.Scan(&cliente.Id, &cliente.Limite, &cliente.Saldo)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(cliente)
		}

		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}

		var transaction Transaction

		json.NewDecoder(r.Body).Decode(&transaction)
		// DoTransaction(transaction, cliente)

		if transaction.Tipo == "c" {
			novo_saldo := cliente.Saldo + transaction.Valor

			resp := Response{Limite: cliente.Limite, Saldo: novo_saldo}
			fmt.Println(novo_saldo, cliente.Saldo, transaction.Valor)

			db.Exec(`UPDATE cliente SET saldo = $1 WHERE id=$2`, novo_saldo, cliente.Id)
			_, err := db.Exec(`INSERT INTO ultimas_transacoes (tipo, descricao, realizada_em, id_cliente, valor) VALUES 
			($1, $2, $3, $4, $5)
			`, transaction.Tipo, transaction.Descricao, time.Now().UTC(), cliente.Id, transaction.Valor)

			if err != nil {
				log.Fatal(err)
			}
			// fmt.Println("tipo c -> result: %d", result)

			w.WriteHeader(http.StatusOK)

			json.NewEncoder(w).Encode(resp)
			return
		} else if transaction.Tipo == "d" {

			if (cliente.Saldo - transaction.Valor) < -cliente.Limite {
				//throw error
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			} else {
				//saldo ok
				novo_saldo := cliente.Saldo - transaction.Valor

				resp := Response{Limite: cliente.Limite, Saldo: novo_saldo}
				fmt.Println(novo_saldo, cliente.Saldo, transaction.Valor)

				db.Exec(`UPDATE cliente SET saldo = $1 WHERE id=$2`, novo_saldo, cliente.Id)
				_, err := db.Exec(`INSERT INTO ultimas_transacoes (tipo, descricao, realizada_em, id_cliente, valor) VALUES 
			($1, $2, $3, $4, $5)
			`, transaction.Tipo, transaction.Descricao, time.Now().UTC(), cliente.Id, transaction.Valor)

				if err != nil {
					log.Fatal(err)
				}
				// fmt.Println("tipo c -> result: %d", result)

				w.WriteHeader(http.StatusOK)

				json.NewEncoder(w).Encode(resp)
				return
			}

		}

		w.WriteHeader(http.StatusInternalServerError)

	}
}

func createCliente(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var cliente Cliente

		err := json.NewDecoder(r.Body).Decode(&cliente)
		if err != nil {
			log.Fatal(err)
		}

		_, err = db.Exec("INSERT INTO cliente (id, limite, saldo) VALUES ($1, $2, $3)", cliente.Id, cliente.Limite, cliente.Saldo)
		if err != nil {
			log.Fatal(err)
		}

		json.NewEncoder(w).Encode(cliente)
	}
}

func Soma(a, b int) int {
	return a + b
}
