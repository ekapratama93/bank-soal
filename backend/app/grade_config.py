# kelas -> {jumlah_soal, durasi_menit}
GRADE_CONFIG: dict[int, dict[str, int]] = {
    1: {"jumlah_soal": 20, "durasi_menit": 60},
    2: {"jumlah_soal": 20, "durasi_menit": 60},
    3: {"jumlah_soal": 25, "durasi_menit": 60},
    4: {"jumlah_soal": 25, "durasi_menit": 60},
    5: {"jumlah_soal": 30, "durasi_menit": 60},
    6: {"jumlah_soal": 30, "durasi_menit": 60},
}

DEFAULT_GRADE_CONFIG = {"jumlah_soal": 30, "durasi_menit": 90}


def get_config(grade: int) -> dict[str, int]:
    return GRADE_CONFIG.get(grade, DEFAULT_GRADE_CONFIG)