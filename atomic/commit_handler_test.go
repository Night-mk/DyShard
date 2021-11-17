/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 8/9/21$ 1:30 AM$
 **/
package atomic

import (
	"bytes"
	crand "crypto/rand"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/trie"
	"testing"
)

// makeProvers creates Merkle trie provers based on different implementations to
// test all variations.
func makeProvers(t *trie.Trie) []func(key []byte) *memorydb.Database {
	var provers []func(key []byte) *memorydb.Database

	// Create a direct trie based Merkle prover
	provers = append(provers, func(key []byte) *memorydb.Database {
		proof := memorydb.New()
		t.Prove(key, 0, proof)
		return proof
	})
	// Create a leaf iterator based Merkle prover
	provers = append(provers, func(key []byte) *memorydb.Database {
		proof := memorydb.New()
		if it := trie.NewIterator(t.NodeIterator(key)); it.Next() && bytes.Equal(key, it.Key) {
			for _, p := range it.Prove() {
				proof.Put(crypto.Keccak256(p), p)
			}
		}
		return proof
	})
	return provers
}

type kv struct {
	k, v []byte
	t    bool
}

func randomTrie(n int) (*trie.Trie, map[string]*kv) {
	trie := new(trie.Trie)
	vals := make(map[string]*kv)
	for i := byte(0); i < 100; i++ {
		value := &kv{common.LeftPadBytes([]byte{i}, 32), []byte{i}, false}
		value2 := &kv{common.LeftPadBytes([]byte{i + 10}, 32), []byte{i}, false}
		trie.Update(value.k, value.v)
		trie.Update(value2.k, value2.v)
		vals[string(value.k)] = value
		vals[string(value2.k)] = value2
	}
	for i := 0; i < n; i++ {
		value := &kv{randBytes(32), randBytes(20), false}
		trie.Update(value.k, value.v)
		vals[string(value.k)] = value
	}
	return trie, vals
}

func randBytes(n int) []byte {
	r := make([]byte, n)
	crand.Read(r)
	return r
}

// 测试state验证
func TestProof(t *testing.T) {
	tr, vals := randomTrie(500)
	root := tr.Hash()
	for i, prover := range makeProvers(tr) {
		for _, kv := range vals {
			proof := prover(kv.k)
			if proof == nil {
				t.Fatalf("prover %d: missing key %x while constructing proof", i, kv.k)
			}
			// 测试verifyProof方法
			val, _, err := trie.VerifyProof(root, kv.k, proof)
			if err != nil {
				t.Fatalf("prover %d: failed to verify proof for key %x: %v\nraw proof: %x", i, kv.k, err, proof)
			}
			if !bytes.Equal(val, kv.v) {
				t.Fatalf("prover %d: verified value mismatch for key %x: have %x, want %x", i, kv.k, val, kv.v)
			}
		}
	}
}

func BenchmarkProve(b *testing.B) {
	trie, vals := randomTrie(100)
	var keys []string
	for k := range vals {
		keys = append(keys, k)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kv := vals[keys[i%len(keys)]]
		proofs := memorydb.New()
		if trie.Prove(kv.k, 0, proofs); proofs.Len() == 0 {
			b.Fatalf("zero length proof for %x", kv.k)
		}
	}
}