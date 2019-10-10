package main

import "testing"

func Test_parseFileName(t *testing.T) {
	fileName := "/var/log/containers/import-api-569fbd9b8-m8stp_import-api-production_import-api-7134a67c5868b1b0d6a964b131af07ef477261a593f1815bd9db7aeeaecafdc4.log"
	expectedPodName := "import-api-569fbd9b8-m8stp"
	expectedPodNamespace := "import-api-production"

	podName, podNamespace := parseFileName(fileName)

	if podName != expectedPodName {
		t.Fatal("want", expectedPodName, "got", podName)
	}
	if podNamespace != expectedPodNamespace {
		t.Fatal("want", expectedPodNamespace, "got", podNamespace)
	}
}
