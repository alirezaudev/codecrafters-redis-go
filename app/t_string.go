package main

func setCommand(c *client, argv [][]byte) error {
	c.db.set(string(argv[1]), string(argv[2]))
	c.addReply(shared.ok)
	return nil
}

func getCommand(c *client, argv [][]byte) error {
	v, ok := c.db.get(string(argv[1]))
	if !ok {
		c.addReplyNull()
		return nil
	}

	c.addReplyBulk([]byte(v))
	return nil
}
