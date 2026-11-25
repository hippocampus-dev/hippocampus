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

	if index := strings.Index(query, "{"); index > 0 {
		data.Series = strings.TrimSpace(query[:index])

		if endIndex := strings.Index(query[index:], "}"); endIndex > 0 {
			data.LabelMatchers = strings.TrimSpace(query[index+1 : index+endIndex])

			matchers := strings.Split(data.LabelMatchers, ",")
			for _, matcher := range matchers {
				matcher = strings.TrimSpace(matcher)
				if matcher == "" {
					continue
				}

				for _, op := range []string{"!=", "!~", "=~", "="} {
					if index := strings.Index(matcher, op); index > 0 {
						key := strings.TrimSpace(matcher[:index])
						value := strings.TrimSpace(matcher[index+len(op):])
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
