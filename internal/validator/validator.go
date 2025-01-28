package validator

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
)

type Validator struct {
	*validator.Validate
	trans ut.Translator
}

type FieldErrors map[string]string

type ValidationErrors struct {
	errors FieldErrors
}

func (v *ValidationErrors) AddFieldError(key, message string) {
	if v.errors == nil {
		v.errors = make(FieldErrors)
	}

	if _, ok := v.errors[key]; ok {
		return
	}

	v.errors[key] = message
}

func (v *ValidationErrors) Error() string {
	if len(v.errors) == 0 {
		return "{}"
	}

	var builder strings.Builder

	_ = json.NewEncoder(&builder).Encode(v.errors)

	return builder.String()
}

func (v *ValidationErrors) FieldErrors() FieldErrors {
	return v.errors
}

func New() *Validator {
	validate := validator.New(validator.WithRequiredStructEnabled())

	english := en.New()

	uni := ut.New(english, english)

	trans, _ := uni.GetTranslator("en")

	_ = en_translations.RegisterDefaultTranslations(validate, trans)

	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		fieldTag := fld.Tag.Get("json")

		if fieldTag == "" {
			fieldTag = fld.Tag.Get("form")
		}

		if fieldTag == "" || fieldTag == "-" {
			return fld.Name
		}
		return strings.Split(fieldTag, ",")[0]
	})

	registerCustomErrorMessages(validate, trans)

	return &Validator{
		Validate: validate,
		trans:    trans,
	}
}

func (v *Validator) Struct(val any, existingValidationErrors ...*ValidationErrors) *ValidationErrors {
	if err := v.Validate.Struct(val); err != nil {
		var validationErrors *ValidationErrors
		if len(existingValidationErrors) != 0 {
			validationErrors = existingValidationErrors[0]
		} else {
			validationErrors = &ValidationErrors{}
		}

		for _, entry := range err.(validator.ValidationErrors) {
			fieldName := entry.Field()
			translatedError := entry.Translate(v.trans)
			validationErrors.AddFieldError(fieldName, translatedError)
		}

		return validationErrors
	}

	return nil
}

// registerCustomErrorMessages registers custom error messages for validation tags
func registerCustomErrorMessages(validate *validator.Validate, trans ut.Translator) {
	_ = validate.RegisterTranslation("required", trans, func(ut ut.Translator) error {
		return ut.Add("required", "{0} is a required field", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("required", fe.Field())
		return t
	})

	_ = validate.RegisterTranslation("email", trans, func(ut ut.Translator) error {
		return ut.Add("email", "{0} must be a valid email address", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("email", fe.Field())
		return t
	})

}
