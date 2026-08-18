package main
import (
	"fmt"
	"reflect"
	"github.com/charmbracelet/bubbles/textarea"
)
func main() {
	ta := textarea.New()
	t := reflect.TypeOf(ta)
	for i := 0; i < t.NumField(); i++ {
		fmt.Println(t.Field(i).Name)
	}
}
