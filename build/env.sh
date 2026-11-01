# Ambiente de compilação cruzada para o X39 (ARMv7, glibc 2.25, Linaro 7.5).
# Uso: source env.sh
B="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"   # esta pasta (build/)
TC="${X39_TOOLCHAIN:-$B/../toolchain/gcc-linaro-7.5.0-2019.12-x86_64_arm-linux-gnueabihf}"
export B TC
export PREFIX=$B/prefix
export PATH=$TC/bin:$PATH
export ARMABI=arm-linux-gnueabihf
export CC=$ARMABI-gcc CXX=$ARMABI-g++ AR=$ARMABI-ar RANLIB=$ARMABI-ranlib STRIP=$ARMABI-strip LD=$ARMABI-ld NM=$ARMABI-nm
# Cortex-A5 (o ATM7051 do X39 é A5, CPU part 0xc05), hard-float com NEON/VFPv4
export ARCHF="-mcpu=cortex-a5 -mfpu=neon-vfpv4 -mfloat-abi=hard"
export CFLAGS="-Os -pipe $ARCHF -I$PREFIX/include"
export CXXFLAGS="$CFLAGS"
export LDFLAGS="-L$PREFIX/lib"
