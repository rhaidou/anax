package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/exchangecommon"
	"time"
)

const NODE_MANAGEMENT_STATUS = "nodemanagementstatus"

func SaveOrUpdateNMPStatus(db *bolt.DB, nmpKey string, status exchangecommon.NodeManagementPolicyStatus) error {
	glog.V(5).Infof(fmt.Sprintf("Saving nmp status %v", status))
	writeErr := db.Update(func(tx *bolt.Tx) error {
		if bucket, err := tx.CreateBucketIfNotExists([]byte(NODE_MANAGEMENT_STATUS)); err != nil {
			return err
		} else if serial, err := json.Marshal(status); err != nil {
			return fmt.Errorf("Failed to serialize node management status: Error: %v", err)
		} else {
			return bucket.Put([]byte(nmpKey), serial)
		}
	})

	return writeErr
}

func DeleteNMPStatus(db *bolt.DB, nmpKey string) (*exchangecommon.NodeManagementPolicyStatus, error) {
	if pol, err := FindNMPStatus(db, nmpKey); err != nil {
		return nil, err
	} else if pol != nil {
		return pol, db.Update(func(tx *bolt.Tx) error {
			if b, err := tx.CreateBucketIfNotExists([]byte(NODE_MANAGEMENT_STATUS)); err != nil {
				return err
			} else if err = b.Delete([]byte(nmpKey)); err != nil {
				return fmt.Errorf("Failed to delete node management policy status %v from the database. Error was: %v", nmpKey, err)
			}
			return nil
		})
	} else {
		return nil, nil
	}
}

func FindNMPStatus(db *bolt.DB, nmpKey string) (*exchangecommon.NodeManagementPolicyStatus, error) {
	var nmStatusRecord *exchangecommon.NodeManagementPolicyStatus
	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(NODE_MANAGEMENT_STATUS)); b != nil {
			nmsSerialRec := b.Get([]byte(nmpKey))
			if nmsSerialRec != nil {
				nmsUnmarsh := exchangecommon.NodeManagementPolicyStatus{}
				if err := json.Unmarshal(nmsSerialRec, &nmsUnmarsh); err != nil {
					return fmt.Errorf("Error unmarshaling node management status: %v", err)
				} else {
					nmStatusRecord = &nmsUnmarsh
				}
			}
		}
		return nil
	})

	if readErr != nil {
		return nil, readErr
	}

	return nmStatusRecord, nil
}

func FindAllNMPStatus(db *bolt.DB) (map[string]*exchangecommon.NodeManagementPolicyStatus, error) {
	statuses := make(map[string]*exchangecommon.NodeManagementPolicyStatus, 0)

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(NODE_MANAGEMENT_STATUS)); b != nil {
			b.ForEach(func(k, v []byte) error {
				var s exchangecommon.NodeManagementPolicyStatus

				if err := json.Unmarshal(v, &s); err != nil {
					return fmt.Errorf("Unable to demarshal node management status record: %v", err)
				} else {
					statuses[string(k)] = &s
				}
				return nil
			})
		}
		return nil
	})

	if readErr != nil {
		return nil, readErr
	}
	return statuses, nil
}

type NMStatusFilter func(exchangecommon.NodeManagementPolicyStatus) bool

func StatusNMSFilter(status string) NMStatusFilter {
	return func(e exchangecommon.NodeManagementPolicyStatus) bool { return e.Status() == status }
}

// return the statuses scheduled for after the given time
func TimeScheduledNMSFilter(t time.Time) NMStatusFilter {
	return func(e exchangecommon.NodeManagementPolicyStatus) bool {
		return t.Before(e.AgentUpgradeInternal.ScheduledUnixTime)
	}
}

func SoftwareUpdateNMSFilter() NMStatusFilter {
	return func(e exchangecommon.NodeManagementPolicyStatus) bool {
		return e.AgentUpgrade.UpgradedVersions.SoftwareVersion != ""
	}
}

func ConfigUpdateNMSFilter() NMStatusFilter {
	return func(e exchangecommon.NodeManagementPolicyStatus) bool {
		return e.AgentUpgrade.UpgradedVersions.ConfigVersion != ""
	}
}

func CertUpdateNMSFilter() NMStatusFilter {
	return func(e exchangecommon.NodeManagementPolicyStatus) bool {
		return e.AgentUpgrade.UpgradedVersions.CertVersion != ""
	}
}

func LatestKeywordNMSFilter() NMStatusFilter {
	return func(e exchangecommon.NodeManagementPolicyStatus) bool {
		return e.AgentUpgradeInternal.LatestMap.SoftwareLatest || e.AgentUpgradeInternal.LatestMap.ConfigLatest || e.AgentUpgradeInternal.LatestMap.CertLatest
	}
}

func FindNMPStatusWithFilters(db *bolt.DB, filters []NMStatusFilter) (map[string]*exchangecommon.NodeManagementPolicyStatus, error) {
	statuses := make(map[string]*exchangecommon.NodeManagementPolicyStatus, 0)

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(NODE_MANAGEMENT_STATUS)); b != nil {
			b.ForEach(func(k, v []byte) error {
				var s exchangecommon.NodeManagementPolicyStatus

				if err := json.Unmarshal(v, &s); err != nil {
					return fmt.Errorf("Unable to demarshal node management status record: %v", err)
				} else {
					include := true
					for _, filter := range filters {
						if !filter(s) {
							include = false
							break
						}
					}
					if include {
						statuses[string(k)] = &s
					}
				}
				return nil
			})
		}
		return nil
	})

	if readErr != nil {
		return nil, readErr
	}
	return statuses, nil
}

func FindWaitingNMPStatuses(db *bolt.DB) (map[string]*exchangecommon.NodeManagementPolicyStatus, error) {
	return FindNMPStatusWithFilters(db, []NMStatusFilter{StatusNMSFilter(exchangecommon.STATUS_NEW)})
}

func FindInitiatedNMPStatuses(db *bolt.DB) (map[string]*exchangecommon.NodeManagementPolicyStatus, error) {
	return FindNMPStatusWithFilters(db, []NMStatusFilter{StatusNMSFilter(exchangecommon.STATUS_INITIATED)})
}

func FindDownloadStartedNMPStatuses(db *bolt.DB) (map[string]*exchangecommon.NodeManagementPolicyStatus, error) {
	return FindNMPStatusWithFilters(db, []NMStatusFilter{StatusNMSFilter(exchangecommon.STATUS_DOWNLOAD_STARTED)})
}

func FindNMPWithLatestKeywordVersion(db *bolt.DB) (map[string]*exchangecommon.NodeManagementPolicyStatus, error) {
	return FindNMPStatusWithFilters(db, []NMStatusFilter{LatestKeywordNMSFilter()})
}

func FindNMPSWithStatuses(db *bolt.DB, statuses []string) (map[string]*exchangecommon.NodeManagementPolicyStatus, error) {
	NMPStatuses := map[string]*exchangecommon.NodeManagementPolicyStatus{}
	for _, status := range statuses {
		matchingStatuses, err := FindNMPStatusWithFilters(db, []NMStatusFilter{StatusNMSFilter(status)})
		if err != nil {
			return nil, err
		}
		for matchingStatusKey, matchingStatus := range matchingStatuses {
			NMPStatuses[matchingStatusKey] = matchingStatus
		}
	}

	return NMPStatuses, nil
}

func FindNodeUpgradeStatusesWithTypeAfterTime(db *bolt.DB, t time.Time, upgradeType string) (map[string]*exchangecommon.NodeManagementPolicyStatus, error) {
	if upgradeType == "software" {
		return FindNMPStatusWithFilters(db, []NMStatusFilter{SoftwareUpdateNMSFilter(), TimeScheduledNMSFilter(t)})
	}
	if upgradeType == "config" {
		return FindNMPStatusWithFilters(db, []NMStatusFilter{ConfigUpdateNMSFilter(), TimeScheduledNMSFilter(t)})
	}
	if upgradeType == "cert" {
		return FindNMPStatusWithFilters(db, []NMStatusFilter{CertUpdateNMSFilter(), TimeScheduledNMSFilter(t)})
	}
	return nil, fmt.Errorf("Unrecognized upgrade type: \"%v\".", upgradeType)
}

func DeleteAllNMPStatuses(db *bolt.DB) error {
	return db.Update(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(NODE_MANAGEMENT_STATUS)); b != nil {
			return tx.DeleteBucket([]byte(NODE_MANAGEMENT_STATUS))
		}
		return nil
	})
}
