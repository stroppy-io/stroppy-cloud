package tenant

import "testing"

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Newbie Lab":           "newbie-lab",
		"Новичок":              "novichok",
		"Команда Яндекс БД":    "komanda-yandeks-bd",
		"Объём  щей":           "obem-shchey",
		"  --Acme_Corp.io--  ": "acme-corp-io",
		"42 team":              "team",
		"":                     "",
		"日本":                   "",
	} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q; want %q", in, got, want)
		}
		if got := Slugify(in); got != "" && !ValidSlug(got) && len(got) >= 3 {
			t.Errorf("Slugify(%q) = %q is not a valid slug", in, got)
		}
	}
}
