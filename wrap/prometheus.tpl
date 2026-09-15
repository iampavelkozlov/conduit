import "time"

// QuerierMetrics decorates every repository method with Prometheus metrics.
type QuerierMetrics struct {
	base    {{.Interface.Type}}
	metrics *Metrics
}

// NewQuerierMetrics wraps a repository without exposing query text or arguments
// as labels. The method name and result have bounded cardinality.
func NewQuerierMetrics(base {{.Interface.Type}}, metrics *Metrics) {{.Interface.Type}} {
	return &QuerierMetrics{base: base, metrics: metrics}
}

{{range $method := .Interface.Methods}}
// {{$method.Name}} implements {{$.Interface.Type}}.
func (w *QuerierMetrics) {{$method.Declaration}} {
	started := time.Now()
	defer func() {
		{{- if $method.ReturnsError}}
		w.metrics.observe("{{$method.Name}}", started, err)
		{{- else}}
		w.metrics.observe("{{$method.Name}}", started, nil)
		{{- end}}
	}()
	{{$method.Pass "w.base."}}
}
{{end}}
