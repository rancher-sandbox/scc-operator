package settingrepo

import (
	"github.com/rancher-sandbox/scc-operator/internal/repos/generic"
	v3ctrl "github.com/rancher-sandbox/scc-operator/pkg/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

var rootSettingRepo *SettingRepository

type SettingRepository generic.NonNamespacedRuntimeObjectRepo[*v3.Setting, *v3.SettingList]

func NewSettingRepository(
	settings v3ctrl.SettingController,
	settingsCache v3ctrl.SettingCache,
) *SettingRepository {
	if rootSettingRepo == nil {
		rootSettingRepo = &SettingRepository{
			Controller: settings,
			Cache:      settingsCache,
		}
		rootSettingRepo.InitIndexers()
	}
	return rootSettingRepo
}

func (repo *SettingRepository) HasSetting(name string) bool {
	_, err := repo.Cache.Get(name)
	return err == nil
}

func (repo *SettingRepository) GetSetting(name string) (*v3.Setting, error) {
	return repo.Cache.Get(name)
}

var _ generic.RuntimeObjectRepository = &SettingRepository{}
