pipeline {
    agent any

    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        stage('Test') {
            steps {
                sh 'go test ./...'
            }
        }

        stage('Build Binary') {
            steps {
                sh 'go build -o kanban ./cmd/kanban'
            }
        }
    }

    post {
        success {
            archiveArtifacts artifacts: 'kanban', fingerprint: true, onlyIfSuccessful: true
        }
        always {
            cleanWs()
        }
    }
}
