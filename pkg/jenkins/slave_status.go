package jenkins

type SlaveStatus struct {
	Offline            bool   `json:"offline"`
	OfflineCauseReason string `json:"offlineCauseReason"`
	TemporarilyOffline bool   `json:"temporarilyOffline"`
	NumExecutors       int    `json:"numExecutors"`
	DisplayName        string `json:"displayName"`
	Description        string `json:"description"`
}
