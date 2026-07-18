package main

func setCommand(c *client, argv [][]byte) error {
	mu.Lock()
	storage[string(argv[1])] = string(argv[2])
	mu.Unlock()

	c.addReply(shared.ok)
	return nil
}

func getCommand(c *client, argv [][]byte) error {
	mu.RLock()
	v, ok := storage[string(argv[1])]
	mu.RUnlock()

	if !ok {
		c.addReplyNull()
		return nil
	}

	c.addReplyBulk([]byte(v))
	return nil
}
