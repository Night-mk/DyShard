/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 8/15/21$ 10:27 PM$
 **/
package shardutil

// Content {shardID, prefix}
//Content represents the data that is stored and verified by the tree. A type that
//implements this interface can be used as an item in the tree.
type Content interface {
	CalculateHash() ([]byte, error)
	Equals(other Content) (bool, error)
	GetPath() string
	GetShardID() uint32
}