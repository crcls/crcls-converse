package channel

func List() []string {
	// TODO: get the list of channels from storage somewhere

	log.Debug("Only the global channel is availble, so far.")
	return []string{"global"}
}
