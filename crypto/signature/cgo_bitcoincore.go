package signature

/*
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <assert.h>
#include "secp256k1.h"
#include "secp256k1_schnorrsig.h" // make sure to build libsecp256k1 with: ./configure --enable-module-schnorrsig --enable-experimental
#include "random.h"

static secp256k1_context *ctx;

static void secp256k1_start() {
	ctx = secp256k1_context_create(SECP256K1_CONTEXT_SIGN | SECP256K1_CONTEXT_VERIFY);
}

static int ecdsa_signature_parse_der_lax(const secp256k1_context* ctx, secp256k1_ecdsa_signature* sig, const unsigned char *input, size_t inputlen) {
    size_t rpos, rlen, spos, slen;
    size_t pos = 0;
    size_t lenbyte;
    unsigned char tmpsig[64] = {0};
    int overflow = 0;

    secp256k1_ecdsa_signature_parse_compact(ctx, sig, tmpsig);

    if (pos == inputlen || input[pos] != 0x30) {
        return 0;
    }
    pos++;

    if (pos == inputlen) {
        return 0;
    }
    lenbyte = input[pos++];
    if (lenbyte & 0x80) {
        lenbyte -= 0x80;
        if (pos + lenbyte > inputlen) {
            return 0;
        }
        pos += lenbyte;
    }

    if (pos == inputlen || input[pos] != 0x02) {
        return 0;
    }
    pos++;

    if (pos == inputlen) {
        return 0;
    }
    lenbyte = input[pos++];
    if (lenbyte & 0x80) {
        lenbyte -= 0x80;
        if (pos + lenbyte > inputlen) {
            return 0;
        }
        while (lenbyte > 0 && input[pos] == 0) {
            pos++;
            lenbyte--;
        }
        if (lenbyte >= sizeof(size_t)) {
            return 0;
        }
        rlen = 0;
        while (lenbyte > 0) {
            rlen = (rlen << 8) + input[pos];
            pos++;
            lenbyte--;
        }
    } else {
        rlen = lenbyte;
    }
    if (rlen > inputlen - pos) {
        return 0;
    }
    rpos = pos;
    pos += rlen;

    if (pos == inputlen || input[pos] != 0x02) {
        return 0;
    }
    pos++;

    if (pos == inputlen) {
        return 0;
    }
    lenbyte = input[pos++];
    if (lenbyte & 0x80) {
        lenbyte -= 0x80;
        if (pos + lenbyte > inputlen) {
            return 0;
        }
        while (lenbyte > 0 && input[pos] == 0) {
            pos++;
            lenbyte--;
        }
        if (lenbyte >= sizeof(size_t)) {
            return 0;
        }
        slen = 0;
        while (lenbyte > 0) {
            slen = (slen << 8) + input[pos];
            pos++;
            lenbyte--;
        }
    } else {
        slen = lenbyte;
    }
    if (slen > inputlen - pos) {
        return 0;
    }
    spos = pos;
    pos += slen;

    while (rlen > 0 && input[rpos] == 0) {
        rlen--;
        rpos++;
    }
    if (rlen > 32) {
        overflow = 1;
    } else {
        memcpy(tmpsig + 32 - rlen, input + rpos, rlen);
    }

    while (slen > 0 && input[spos] == 0) {
        slen--;
        spos++;
    }
    if (slen > 32) {
        overflow = 1;
    } else {
        memcpy(tmpsig + 64 - slen, input + spos, slen);
    }

    if (!overflow) {
        overflow = !secp256k1_ecdsa_signature_parse_compact(ctx, sig, tmpsig);
    }
    if (overflow) {
        memset(tmpsig, 0, 64);
        secp256k1_ecdsa_signature_parse_compact(ctx, sig, tmpsig);
    }
    return 1;
}

static int cgo_generate_key(unsigned char *seckey32){
    if (!fill_random(seckey32, 32)) {
            printf("Failed to generate randomness\n");
            return 1;
        }
}

static void cgo_create_secp256k1_keypair(secp256k1_keypair *keypair, const unsigned char *seckey32){
    int ret=0;
    ret=secp256k1_keypair_create(ctx,keypair,seckey32);
    if (ret==0){
        puts("wrong keypair");
    }
}

static void cgo_secp256k1_keypair_xonly_pub(secp256k1_xonly_pubkey *pubkey,secp256k1_keypair *keypair){
    int ret=0;
    ret=secp256k1_keypair_xonly_pub(ctx, pubkey, NULL, keypair);
    if (ret==0){
        puts("wrong keypair");
    }
}

static void cgo_secp256k1_xonly_pubkey_serialize(unsigned char *pubkey32,const secp256k1_xonly_pubkey *pubkey){
    int ret=0;
    ret=secp256k1_xonly_pubkey_serialize(ctx, pubkey32,pubkey);
    if (ret==0){
        puts("wrong keypair");
    }
}

static int cgo_secp256k1_ec_pubkey_create(unsigned char *pubkey33,size_t pklen,unsigned char *seckey){
    int ret=0;
    secp256k1_pubkey pubkey;
    ret=secp256k1_ec_pubkey_create(ctx, &pubkey, seckey);
    if (ret!=1){
        return ret;
    }
    ret=secp256k1_ec_pubkey_serialize(ctx, pubkey33, &pklen, &pubkey, SECP256K1_EC_COMPRESSED);
    return ret;
}

static void ecdsa_sign(unsigned char *msg,unsigned char *seckey,int sklen,unsigned char *sig,size_t siglen) {
    int ret;
    secp256k1_ecdsa_signature signature;
    unsigned char auxiliary_rand[32];
    if (!fill_random(auxiliary_rand, sizeof(auxiliary_rand))) {
        printf("Failed to generate randomness\n");
    }
    ret=secp256k1_ecdsa_sign(ctx,&signature,msg,seckey,NULL,auxiliary_rand);
    if (ret==0) {
        puts("wrong sign");
    }
    ret=secp256k1_ecdsa_signature_serialize_compact(ctx,sig,&signature);
    if (ret==0) {
        puts("wrong serialize");
    }

}

static int secp256k1_verify(unsigned char *msg, unsigned char *sig, int siglen, unsigned char *pk, int pklen) {
	int result;
    secp256k1_pubkey pubkey;
	secp256k1_ecdsa_signature _sig;

	if (!secp256k1_ec_pubkey_parse(ctx, &pubkey, pk, pklen)) {
		return -2;
	}
    if (!secp256k1_ecdsa_signature_parse_compact(ctx, &_sig, sig)) {
		return -3;
	}

	//secp256k1_ecdsa_signature_normalize(ctx, &_sig, &_sig);
	result = secp256k1_ecdsa_verify(ctx, &_sig, msg, &pubkey);

	return result;
}

static void schnorr_sign(unsigned char *msg,secp256k1_keypair keypair,unsigned char *sig) {
    int ret;
    unsigned char auxiliary_rand[32];
    if (!fill_random(auxiliary_rand, sizeof(auxiliary_rand))) {
        printf("Failed to generate randomness\n");
    }
    ret=secp256k1_schnorrsig_sign(ctx,sig,msg,&keypair,NULL,auxiliary_rand);
    if (ret==0) {
        puts("wrong sign");
    }
}

static int schnorr_verify(unsigned char *msg, unsigned char *sig, unsigned char *pk) {
	secp256k1_xonly_pubkey pubkey;
	if (!secp256k1_xonly_pubkey_parse(ctx, &pubkey, pk)) return 0;
	//printf("pubkey: %02x%02x.. ==> %02x%02x... %02x%02x...\n", pk[0], pk[1], pubkey.data[0], pubkey.data[1], pubkey.data[32], pubkey.data[33]);
	return secp256k1_schnorrsig_verify(ctx, sig, msg, &pubkey);
}

static int schnorr_sign_aggregate(unsigned char *signature_final,unsigned char *sigs,unsigned char *pks,const unsigned char *msg,int32_t memlen){
    const unsigned char **sigs2d=(const unsigned char**)malloc(sizeof(*sigs2d)*memlen);
    const secp256k1_xonly_pubkey **pk = (const secp256k1_xonly_pubkey **)malloc(memlen * sizeof(*pk));
    for (int i=0;i<memlen;i++){
        sigs2d[i]=&sigs[i*64];
    }
    for(int i=0;i<memlen;i++){
        secp256k1_xonly_pubkey *pk_nonconst = (secp256k1_xonly_pubkey *)malloc(sizeof(*pk_nonconst));
        if(!secp256k1_xonly_pubkey_parse(ctx, pk_nonconst, &pks[i*32])){
            puts("wrong pk\n");
            return 0;
        }
        pk[i] = pk_nonconst;
    }
    return secp256k1_schnorrsig_aggrate_sign(ctx,signature_final,memlen*32+32,sigs2d,msg,pk,memlen);
}

static int secp256k1_schnorrsig_aggsig_verify(unsigned char *aggsig,unsigned char *pks,const unsigned char *msg,size_t msglen,int32_t memlen){
    secp256k1_scratch_space *scratch;
    int ret;
    const secp256k1_xonly_pubkey **pk = (const secp256k1_xonly_pubkey **)malloc(memlen * sizeof(*pk));
    for(int i=0;i<memlen;i++){
        secp256k1_xonly_pubkey *pk_nonconst = (secp256k1_xonly_pubkey *)malloc(sizeof(*pk_nonconst));
        if(!secp256k1_xonly_pubkey_parse(ctx, pk_nonconst, &pks[i*32]))
        {
            puts("wrong pk\n");
            return 0;
        }
        pk[i] = pk_nonconst;

    }
    scratch = secp256k1_scratch_space_create(ctx, 700 * 1024 * 1024);
    ret= secp256k1_schnorrsig_aggrate_verify(ctx, scratch,aggsig,memlen*32+32,msg,pk,memlen);

    for(int i=0;i<memlen;i++){
        free((secp256k1_xonly_pubkey *)pk[i]);
    }
    free((secp256k1_xonly_pubkey **)pk);
    secp256k1_scratch_space_destroy(ctx, scratch);
    return ret;
}

void cgo_schnorr_get_pubkey(unsigned char *pk,unsigned char *sk){
	secp256k1_keypair keypair;
	secp256k1_xonly_pubkey pubkey;
	int ret=0;
    ret=secp256k1_keypair_create(ctx,&keypair,sk);
    assert(ret);
	ret=secp256k1_keypair_xonly_pub(ctx, &pubkey, NULL, &keypair);
	assert(ret);
	ret=secp256k1_xonly_pubkey_serialize(ctx, pk, &pubkey);
	assert(ret);
}

void cgo_ecdsa_get_pubkey(unsigned char *pk,unsigned char *sk){
	size_t len;
	int return_val;
    secp256k1_pubkey pubkey;
	return_val = secp256k1_ec_pubkey_create(ctx, &pubkey, sk);
    assert(return_val);
	len=33;
	return_val = secp256k1_ec_pubkey_serialize(ctx, pk, &len, &pubkey, SECP256K1_EC_COMPRESSED);
    assert(return_val);
}

*/
import "C"
import (
	"encoding/pem"
	"os"
	"unsafe"
)

const schnorr_signature = 1
const ecdsa_signature = 2

type Keypair = C.secp256k1_keypair

type SignKey struct {
	sign_type int
	sk        []byte
	Pk        []byte
	keypair   C.secp256k1_keypair
}

func Generate_Key() []byte {
	sk := make([]byte, 32)
	ret := C.cgo_generate_key((*C.uchar)(unsafe.Pointer(&sk[0])))
	if ret == 0 {
		panic("error when generate key")
	}
	return sk
}

func SchnorrGetPubKey(seckey []byte) []byte {
	var Cpk [33]C.uchar
	C.cgo_schnorr_get_pubkey((*C.uchar)(unsafe.Pointer(&Cpk[0])), (*C.uchar)(unsafe.Pointer(&seckey[0])))

	var pubkey []byte
	for _, v := range Cpk {
		pubkey = append(pubkey, byte(v))
	}
	return pubkey
}

func SchnorrGetKeypair(seckey []byte) Keypair {
	var keypair Keypair
	C.cgo_create_secp256k1_keypair(&keypair, (*C.uchar)(unsafe.Pointer(&seckey[0])))
	return keypair
}

func ECDSAGetPubKey(seckey []byte) []byte {
	var Cpk [32]C.uchar
	C.cgo_ecdsa_get_pubkey((*C.uchar)(unsafe.Pointer(&Cpk[0])), (*C.uchar)(unsafe.Pointer(&seckey[0])))

	var pubkey []byte
	for _, v := range Cpk {
		pubkey = append(pubkey, byte(v))
	}
	return pubkey
}

func KeyStore(key []byte, keytype string, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	block := &pem.Block{
		Type:  keytype,
		Bytes: key,
	}
	err = pem.Encode(file, block)
	if err != nil {
		return err
	}
	file.Close()
	return nil
}

func NewSignKey(seckey []byte, sign_type int) SignKey {
	if sign_type == 1 {
		var keypair C.secp256k1_keypair
		C.cgo_create_secp256k1_keypair(&keypair, (*C.uchar)(unsafe.Pointer(&seckey[0])))
		var pk C.secp256k1_xonly_pubkey
		C.cgo_secp256k1_keypair_xonly_pub(&pk, &keypair)

		var Cpk [32]C.uchar
		C.cgo_secp256k1_xonly_pubkey_serialize((*C.uchar)(unsafe.Pointer(&Cpk[0])), &pk)

		var pubkey []byte
		for _, v := range Cpk {
			pubkey = append(pubkey, byte(v))
		}
		var signkey SignKey
		signkey.sign_type = sign_type
		signkey.sk = seckey
		signkey.Pk = pubkey
		signkey.keypair = keypair
		return signkey
	} else if sign_type == 2 {
		var Cpk [33]C.uchar
		var Cpklen C.size_t = 33
		ret := C.cgo_secp256k1_ec_pubkey_create((*C.uchar)(unsafe.Pointer(&Cpk[0])), Cpklen, (*C.uchar)(unsafe.Pointer(&seckey[0])))
		if int(ret) != 1 {
			panic("wrong pk")
		}
		var pubkey []byte
		for _, v := range Cpk {
			pubkey = append(pubkey, byte(v))
		}
		var signkey SignKey
		signkey.sign_type = sign_type
		signkey.sk = seckey
		signkey.Pk = pubkey
		return signkey
	} else {
		panic("wrong signature type")
	}

}

func (key *SignKey) Sign(msg []byte) []byte {
	switch key.sign_type {
	case ecdsa_signature:
		return EC_Sign(key.sk, msg)
	case schnorr_signature:
		return Schnorr_Sign(key.keypair, msg)
	default:
		return nil
	}

}

func EC_Sign(skey, msg []byte) []byte {
	var Csig [64]C.uchar
	var Csiglen C.size_t = 64
	C.ecdsa_sign((*C.uchar)(unsafe.Pointer(&msg[0])), (*C.uchar)(unsafe.Pointer(&skey[0])), C.int(len(skey)), &Csig[0], Csiglen)
	var sig []byte
	//fmt.Println(Csig)
	for _, v := range Csig {
		sig = append(sig, byte(v))

	}
	return sig
}

// EC_Verify verifies an ECDSA signature.
func EC_Verify(pkey, sign, hash []byte) int {
	return int(C.secp256k1_verify((*C.uchar)(unsafe.Pointer(&hash[0])),
		(*C.uchar)(unsafe.Pointer(&sign[0])), C.int(len(sign)),
		(*C.uchar)(unsafe.Pointer(&pkey[0])), C.int(len(pkey))))
}

func Schnorr_Sign(keypair C.secp256k1_keypair, msg []byte) []byte {
	var Csig [64]C.uchar
	C.schnorr_sign((*C.uchar)(unsafe.Pointer(&msg[0])), keypair, &Csig[0])
	var sig []byte
	for _, v := range Csig {
		sig = append(sig, byte(v))

	}
	return sig
}

func Schnorr_Verify(pkey, sign, msg []byte) int {
	return int(C.schnorr_verify((*C.uchar)(unsafe.Pointer(&msg[0])),
		(*C.uchar)(unsafe.Pointer(&sign[0])), (*C.uchar)(unsafe.Pointer(&pkey[0]))))
}

func Schnorr_Sign_Aggregate(sigs []byte, pks []byte, msg []byte, memlen int32) []byte {
	aggsig := make([]byte, memlen*32+32)
	ret := C.schnorr_sign_aggregate((*C.uchar)(unsafe.Pointer(&aggsig[0])), (*C.uchar)(unsafe.Pointer(&sigs[0])), (*C.uchar)(unsafe.Pointer(&pks[0])), (*C.uchar)(unsafe.Pointer(&msg[0])), (C.int)(memlen))

	if ret == 0 {
		panic("wrong aggregate schnorr signature")
	}

	return aggsig
}

func Schnorr_Sign_Aggregate_verify(aggsig []byte, pks []byte, msg []byte, memlen int32) int {
	msglen := len(msg)
	ret := C.secp256k1_schnorrsig_aggsig_verify((*C.uchar)(unsafe.Pointer(&aggsig[0])), (*C.uchar)(unsafe.Pointer(&pks[0])), (*C.uchar)(unsafe.Pointer(&msg[0])), C.ulong(msglen), C.int(memlen))
	//fmt.Println("result of verify ", ret)
	return int(ret)
}

func init() {
	C.secp256k1_start()
}

