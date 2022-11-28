package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"

	"cloud.google.com/go/pubsub"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Template struct {
	Templates *template.Template
}

// TemplateRenderer is a custom html/template renderer for Echo framework
type TemplateRenderer struct {
	templates *template.Template
}

// implement echo interface
func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.Templates.ExecuteTemplate(w, name, data)
}

func main() {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	t := &Template{
		Templates: template.Must(template.ParseGlob("web/home.html")),
	}

	e.Renderer = t
	e.GET("/", func(c echo.Context) error {

		//passing in the template name (not file name)
		return c.Render(http.StatusOK, "home", nil)
	})

	//respond to POST requests and send message to callback URL
	e.POST("/supportrequest", func(c echo.Context) error {

		supportreq := c.FormValue("supportreq")
		fmt.Println(supportreq)

		//create client
		ctx := context.Background()
		client, err := pubsub.NewClient(ctx, "seroter-project-base")
		if err != nil {
			return fmt.Errorf("pubsub: New Client: %v", err)
		}

		defer client.Close()

		//publish to topic async
		t := client.Topic("ticket-router")
		result := t.Publish(ctx, &pubsub.Message{Data: []byte(supportreq)})

		//wait for result
		id, err := result.Get(ctx)
		if err != nil {
			return fmt.Errorf("pubsub: result.Get: %v", err)
		}

		fmt.Printf("published a message: message ID: %v", id)

		//write to activity log file
		f, lerr := os.OpenFile("/logs/msgs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if lerr != nil {
			return fmt.Errorf("error opening or creating file: %v\n", lerr)
		}

		defer f.Close()

		if _, werr := f.WriteString("message: " + supportreq + "\n"); werr != nil {
			return fmt.Errorf("error writing message: %v", werr)
		}

		//write to result output file
		f2, oerr := os.OpenFile("/acks/ids.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if oerr != nil {
			return fmt.Errorf("error opening or creating file: %v\n", oerr)
		}

		defer f2.Close()

		if _, werr := f2.WriteString("ack: " + id + "\n"); werr != nil {
			return fmt.Errorf("error writing ack: %v", werr)
		}

		return c.Render(http.StatusOK, "home", nil)
	})

	//simple startup
	e.Logger.Fatal(e.Start(":8080"))

}
