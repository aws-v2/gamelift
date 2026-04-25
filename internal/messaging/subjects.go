package messaging

// GetS3StoredSubject returns the subject for S3 upload completion events.
func GetS3StoredSubject() Subject {
	return Subject{
		Service:    "s3",
		Domain:     "game",
		ActionType: "stored",
	}
}

// GetEC2ProvisionSubject returns the subject to trigger EC2 VM provisioning.
func GetEC2ProvisionSubject() Subject {
	return Subject{
		Service:    "ec2",
		Domain:     "vm",
		ActionType: "provision",
	}
}

// GetInstanceLifecycleSubject returns the subject for VM lifecycle progress streams.
func GetInstanceLifecycleSubject() Subject {
	return Subject{
		Service:    "compute",
		Domain:     "instance",
		ActionType: "lifecycle",
	}
}

// GetGameStateBroadcastSubject returns the subject for game state synchronization.
func GetGameStateBroadcastSubject() Subject {
	return Subject{
		Service:    "game",
		Domain:     "state",
		ActionType: "broadcast",
	}
}

// GetProvisionGameSubject returns the subject for internal provisioning requests.
func GetProvisionGameSubject() Subject {
	return Subject{
		Service:    "provisioning",
		Domain:     "game",
		ActionType: "provision",
	}
}

// GetGameReadySubject returns the subject for game ready notifications.
func GetGameReadySubject() Subject {
	return Subject{
		Service:    "provisioning",
		Domain:     "game",
		ActionType: "ready",
	}
}