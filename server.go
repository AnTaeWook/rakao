package main

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()

	e.GET("/chat", SetCookie)
	e.GET("/chat/refresh", RefreshCookie)

	e.Logger.Fatal(e.Start(":8080"))
}

func SetCookie(c echo.Context) error {
	_, err := c.Cookie("rakao_id")
	if err == nil {
		return c.NoContent(http.StatusNoContent)
	}
	cookie := createIdCookie()

	c.SetCookie(cookie)

	return c.NoContent(http.StatusNoContent)
}

func RefreshCookie(c echo.Context) error {
	cookie := createIdCookie()

	c.SetCookie(cookie)

	return c.NoContent(http.StatusNoContent)
}

func createIdCookie() *http.Cookie {
	cookie := new(http.Cookie)
	cookie.Name = "rakao_id"
	cookie.Value = uuid.New().String()
	cookie.Expires = time.Now().Add(24 * time.Hour)
	cookie.Path = "/"
	cookie.HttpOnly = true
	return cookie
}
