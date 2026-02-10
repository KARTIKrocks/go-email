package email

import (
	"bytes"
	htmltemplate "html/template"
	"os"
	texttemplate "text/template"
)

// Template represents an email template
type Template struct {
	name     string
	subject  string
	textTmpl *texttemplate.Template
	htmlTmpl *htmltemplate.Template
}

// NewTemplate creates a new email template
func NewTemplate(name string) *Template {
	return &Template{
		name: name,
	}
}

// SetSubject sets the subject template
func (t *Template) SetSubject(subject string) *Template {
	t.subject = subject
	return t
}

// SetTextTemplate sets the plain text template
func (t *Template) SetTextTemplate(tmpl string) (*Template, error) {
	parsed, err := texttemplate.New(t.name + "_text").Parse(tmpl)
	if err != nil {
		return nil, err
	}
	t.textTmpl = parsed
	return t, nil
}

// SetHTMLTemplate sets the HTML template
func (t *Template) SetHTMLTemplate(tmpl string) (*Template, error) {
	parsed, err := htmltemplate.New(t.name + "_html").Parse(tmpl)
	if err != nil {
		return nil, err
	}
	t.htmlTmpl = parsed
	return t, nil
}

// Render renders the template with data
func (t *Template) Render(data any) (*Email, error) {
	email := NewEmail()

	// Render subject
	if t.subject != "" {
		subjTmpl, err := texttemplate.New("subject").Parse(t.subject)
		if err != nil {
			return nil, err
		}

		var subjBuf bytes.Buffer
		if err := subjTmpl.Execute(&subjBuf, data); err != nil {
			return nil, err
		}
		email.Subject = subjBuf.String()
	}

	// Render text body
	if t.textTmpl != nil {
		var textBuf bytes.Buffer
		if err := t.textTmpl.Execute(&textBuf, data); err != nil {
			return nil, err
		}
		email.Body = textBuf.String()
	}

	// Render HTML body
	if t.htmlTmpl != nil {
		var htmlBuf bytes.Buffer
		if err := t.htmlTmpl.Execute(&htmlBuf, data); err != nil {
			return nil, err
		}
		email.HTMLBody = htmlBuf.String()
	}

	return email, nil
}

// LoadTemplateFromFile loads a template from a file.
// The file content is used as the HTML template.
func LoadTemplateFromFile(name, path string) (*Template, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	tmpl := NewTemplate(name)
	if _, err := tmpl.SetHTMLTemplate(string(content)); err != nil {
		return nil, err
	}

	return tmpl, nil
}
