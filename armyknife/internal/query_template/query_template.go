package query_template

import (
	"bytes"
	"strings"
	"text/template"

	"golang.org/x/xerrors"
)

type TemplateData struct {
	Series            string
	LabelMatchers     string
	GroupBy           string
	GroupBySlice      []string
	LabelValuesByName map[string]string
}

func Process(queryTemplate string, data TemplateData) (string, error) {
	tmpl, err := template.New("query").Delims("<<", ">>").Parse(queryTemplate)
	if err != nil {
		return "", xerrors.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", xerrors.Errorf("failed to execute template: %w", err)
	}

	return strings.TrimSpace(buf.String()), nil
}

func ExtractTemplateDataFromQuery(query string) (*TemplateData, error) {
	data := &TemplateData{
		LabelValuesByName: make(map[string]string),
	}

	if idx := strings.Index(query, "{"); idx > 0 {
		data.Series = strings.TrimSpace(query[:idx])

		if endIdx := strings.Index(query[idx:], "}"); endIdx > 0 {
			data.LabelMatchers = strings.TrimSpace(query[idx+1 : idx+endIdx])

			matchers := strings.Split(data.LabelMatchers, ",")
			for _, matcher := range matchers {
				matcher = strings.TrimSpace(matcher)
				if matcher == "" {
					continue
				}

				for _, op := range []string{"!=", "!~", "=~", "="} {
					if idx := strings.Index(matcher, op); idx > 0 {
						key := strings.TrimSpace(matcher[:idx])
						value := strings.TrimSpace(matcher[idx+len(op):])
						value = strings.Trim(value, `"`)
						data.LabelValuesByName[key] = value
						break
					}
				}
			}
		}
	} else {
		data.Series = strings.TrimSpace(query)
	}

	return data, nil
}
