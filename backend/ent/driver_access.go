package ent

import "entgo.io/ent/dialect"

func (c *Client) Driver() dialect.Driver {
	return c.driver
}

func (tx *Tx) Driver() dialect.Driver {
	return tx.driver
}
