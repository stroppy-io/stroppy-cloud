package compile

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
)

func (c *compilation) placementZones(settings map[string]any) ([]string, error) {
	if c.in.Database.Kind != catalog.YDB || ydbErasure(strParam(c.params(), "fault_tolerance", "none")) != "mirror-3-dc" {
		return nil, nil
	}
	if c.in.ProviderKind != "yandex" {
		return nil, errs.Newf(errs.CodeInvalid, "YDB mirror-3-dc placement is implemented for Yandex Cloud only")
	}
	return c.threeYandexZones(settings)
}

func (c *compilation) threeYandexZones(settings map[string]any) ([]string, error) {
	var zones []string
	if raw, ok := settings["zones"].([]any); ok {
		for _, z := range raw {
			value, ok := z.(string)
			if !ok {
				return nil, errs.Newf(errs.CodeInvalid, "Distributed YDB zones must be strings")
			}
			zones = append(zones, value)
		}
	} else {
		zones = append(zones, c.location(settings))
		for _, location := range c.in.Provider.Locations {
			if location.ID != zones[0] && len(zones) < 3 {
				zones = append(zones, location.ID)
			}
		}
	}
	if len(zones) != 3 {
		return nil, errs.Newf(errs.CodeInvalid, "Distributed YDB needs exactly three physical provider zones")
	}
	seen := map[string]bool{}
	for _, zone := range zones {
		valid := false
		for _, location := range c.in.Provider.Locations {
			if location.ID == zone {
				valid = true
			}
		}
		if !valid || seen[zone] {
			return nil, errs.Newf(errs.CodeInvalid, "Distributed YDB zones must be distinct supported provider locations: %q", zone)
		}
		seen[zone] = true
	}
	return zones, nil
}

func (c *compilation) machineLocation(name string) string {
	for _, machine := range c.out.Spec.Machines {
		if machine.Name == name {
			return machine.Location
		}
	}
	return ""
}
