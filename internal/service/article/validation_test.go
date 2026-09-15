package article

import (
	"testing"

	"conduit/internal/models"

	"github.com/stretchr/testify/require"
)

func TestValidateUpdate(t *testing.T) {
	empty := ""
	valid := "updated"
	tags := []string{}

	tests := map[string]struct {
		update  models.UpdateArticle
		wantErr bool
	}{
		"omitted fields":        {},
		"valid supplied fields": {update: models.UpdateArticle{Title: &valid, TitleSet: true, TagList: &tags, TagListSet: true}},
		"null title":            {update: models.UpdateArticle{TitleSet: true}, wantErr: true},
		"empty title":           {update: models.UpdateArticle{Title: &empty, TitleSet: true}, wantErr: true},
		"null description":      {update: models.UpdateArticle{DescriptionSet: true}, wantErr: true},
		"empty body":            {update: models.UpdateArticle{Body: &empty, BodySet: true}, wantErr: true},
		"null tag list":         {update: models.UpdateArticle{TagListSet: true}, wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateUpdate(tc.update)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
