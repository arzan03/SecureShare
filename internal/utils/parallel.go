package utils

import (
	"sync"
)

// ParallelTask represents a generic task that can be executed in parallel
type ParallelTask func() (interface{}, error)

// RunParallelTasks executes multiple tasks in parallel and returns results
func RunParallelTasks(tasks []ParallelTask) ([]interface{}, []error) {
	var wg sync.WaitGroup
	results := make([]interface{}, len(tasks))
	errors := make([]error, len(tasks))

	wg.Add(len(tasks))
	for i, task := range tasks {
		go func(index int, t ParallelTask) {
			defer wg.Done()
			result, err := t()
			results[index] = result
			errors[index] = err
		}(i, task)
	}

	wg.Wait()
	return results, errors
}

// WorkerPool creates a worker pool for processing tasks
type WorkerPool struct {
	maxWorkers int
	taskChan   chan func()
	wg         sync.WaitGroup
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(maxWorkers int) *WorkerPool {
	pool := &WorkerPool{
		maxWorkers: maxWorkers,
		taskChan:   make(chan func(), maxWorkers*2), // Buffer for tasks
	}

	// Start workers
	for i := 0; i < maxWorkers; i++ {
		go pool.worker()
	}

	return pool
}

// worker processes tasks from the task channel
func (p *WorkerPool) worker() {
	for task := range p.taskChan {
		task()
		p.wg.Done()
	}
}

// AddTask adds a task to the worker pool
func (p *WorkerPool) AddTask(task func()) {
	p.wg.Add(1)
	p.taskChan <- task
}

// Wait waits for all tasks to complete
func (p *WorkerPool) Wait() {
	p.wg.Wait()
}

// Close closes the worker pool
func (p *WorkerPool) Close() {
	close(p.taskChan)
}
