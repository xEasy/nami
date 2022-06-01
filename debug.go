package nami

import (
	"fmt"
	"net/http"
	"text/template"
)

const debugText = `<html>
	<body>
	<title>GeeRPC Services</title>
	{{range .}}
	<hr>
	Service {{.Name}}
	<hr>
		<table>
		<th align=center>Method</th><th align=center>Calls</th>
		{{range $name, $mtype := .Method}}
			<tr>
			<td align=left font=fixed>{{$name}}({{$mtype.ArgType}}, {{$mtype.ReplyType}}) error</td>
			<td align=center>{{$mtype.NumCalls}}</td>
			</tr>
		{{end}}
		</table>
	{{end}}
	</body>
	</html>`

var debug = template.Must(template.New("Nami debug").Parse(debugText))

type debugHTTP struct {
	*Server
}

type debugService struct {
	Name   string
	Method map[string]*methodType
}

func (ds debugHTTP) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var services []debugService
	ds.serviceMap.Range(func(namei, svci any) bool {
		svc := svci.(*service)
		name := namei.(string)
		services = append(services, debugService{Name: name, Method: svc.method})
		return true
	})

	err := debug.Execute(w, services)
	if err != nil {
		_, _ = fmt.Fprintln(w, "rpc service: Execute template error: ", err)
	}
}
