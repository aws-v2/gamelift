package messaging

// GetS3StoredSubject returns the subject for S3 upload completion events.
func GetS3StoredSubject(env string) Subject {
	return Subject{
		Env:        env,
		Service:    "s3",
		Version:    "v1",
		Domain:     "game",
		ActionType: "stored",
	}
}

// GetEC2ProvisionSubject returns the subject to trigger EC2 VM provisioning.
func GetEC2ProvisionSubject(env string) Subject {
	return Subject{
		Env:        env,
		Service:    "ec2",
		Version:    "v1",
		Domain:     "vm",
		ActionType: "provision",
	}
}

// GetInstanceLifecycleSubject returns the subject for VM lifecycle progress streams.
func GetInstanceLifecycleSubject(env string) Subject {
	return Subject{
		Env:        env,
		Service:    "compute",
		Version:    "v1",
		Domain:     "instance",
		ActionType: "lifecycle",
	}
}

// GetGameStateBroadcastSubject returns the subject for game state synchronization.
func GetGameStateBroadcastSubject(env string) Subject {
	return Subject{
		Env:        env,
		Service:    "game",
		Version:    "v1",
		Domain:     "state",
		ActionType: "broadcast",
	}
}

// GetProvisionGameSubject returns the subject for internal provisioning requests.
func GetProvisionGameSubject(env string) Subject {
	return Subject{
		Env:        env,
		Service:    "provisioning",
		Version:    "v1",
		Domain:     "game",
		ActionType: "provision",
	}
}

// GetGameReadySubject returns the subject for game ready notifications.
func GetGameReadySubject(env string) Subject {
	return Subject{
		Env:        env,
		Service:    "provisioning",
		Version:    "v1",
		Domain:     "game",
		ActionType: "ready",
	}
}
